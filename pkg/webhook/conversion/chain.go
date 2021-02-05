package conversion

import (
	"strings"

	"github.com/flant/shell-operator/pkg/utils/string_helper"
)

type ChainStorage struct {
	Chains map[string]*Chain
}

func NewChainStorage() *ChainStorage {
	return &ChainStorage{
		Chains: make(map[string]*Chain),
	}
}

type Chain struct {
	// A cache of calculated paths. A key is an arbitrary path and value is a sequence of base paths.
	PathsCache map[Rule][]Rule
	// An index to search for base paths by given From and To.
	BaseFromToIndex map[string]map[string]struct{}
}

// Access a Chain by CRD full name (e.g. crontab.stable.example.com)
func (cs ChainStorage) Get(crdName string) *Chain {
	if _, ok := cs.Chains[crdName]; !ok {
		cs.Chains[crdName] = &Chain{
			PathsCache:      make(map[Rule][]Rule),
			BaseFromToIndex: make(map[string]map[string]struct{}),
		}
	}
	return cs.Chains[crdName]
}

// Put adds a base path defined by Rule to a cache and updates a FromTo index.
func (c *Chain) Put(rule Rule) {
	// Save rule in cache as a base path.
	c.PathsCache[rule] = []Rule{rule}
	// Update from -> to index.
	if _, ok := c.BaseFromToIndex[rule.FromVersion]; !ok {
		c.BaseFromToIndex[rule.FromVersion] = make(map[string]struct{})
	}
	c.BaseFromToIndex[rule.FromVersion][rule.ToVersion] = struct{}{}
}

// Calculations
// FindConversionChain returns an array of ConverionsRules that should be
func (cs ChainStorage) FindConversionChain(crdName string, rule Rule) []Rule {
	chain, ok := cs.Chains[crdName]
	if !ok {
		return nil
	}

	// Return if there is no path to a desired version, trimmed or full.
	if !chain.HasTargetVersion(rule.ToVersion) {
		return nil
	}

	for {
		p := chain.SearchPathForRule(rule)
		if len(p) > 0 {
			return p
		}

		// A temporary map to fill the cache later with more calculated paths.
		newPaths := map[Rule][]Rule{}

		// Try only paths that starts from a similar FromVersion as in input rule.
		for _, ruleToCheck := range chain.RulesWithSimilarFromVersion(rule) {

			if ruleToCheck.ShortToVersion() == rule.ShortFromVersion() {
				// Ignore loops.
				continue
			}

			// toVersion in ruleIDToCheck is a new start. Get toVersions available starting from it.
			for _, nextRule := range chain.NextRules(ruleToCheck.ToVersion) {
				newRule := Rule{
					FromVersion: rule.FromVersion,
					ToVersion:   nextRule.ToVersion,
				}

				if newRule.ShortToVersion() == rule.ShortFromVersion() {
					// Ignore loops.
					continue
				}

				newPath := append(chain.PathsCache[ruleToCheck], nextRule)

				// This path is already discovered.
				p := chain.SearchPathForRule(newRule)
				if len(p) != 0 {
					continue
				}

				newPaths[newRule] = newPath
			}
		}

		// break if no new paths are discovered.
		if len(newPaths) == 0 {
			break
		}

		// Put new paths in cache.
		for pathKey, path := range newPaths {
			chain.PathsCache[pathKey] = path
		}
	}

	return nil
}

// SearchPathForRule returns an array of base paths for an arbitrary rule.
func (c Chain) SearchPathForRule(rule Rule) []Rule {
	pathKeys := []Rule{}
	for cachedPath := range c.PathsCache {
		// Check for exact match.
		if rule.ToVersion == cachedPath.ToVersion && rule.FromVersion == cachedPath.FromVersion {
			return c.PathsCache[cachedPath]
		}
		if VersionsMatched(rule.ToVersion, cachedPath.ToVersion) && VersionsMatched(rule.FromVersion, cachedPath.FromVersion) {
			pathKeys = append(pathKeys, cachedPath)
		}
	}

	// Oops. No similar paths.
	if len(pathKeys) == 0 {
		return nil
	}
	// Return if only one similar path is found.
	if len(pathKeys) == 1 {
		return c.PathsCache[pathKeys[0]]
	}

	// Try to find a more stricter path. Prefer a path with toVersion with group.
	// Note that length of the pathKeys slice should not be more than 3:
	// a) Possible variants of cached paths are:
	//     1. group1/v1->v2
	//     2. group1/v1->group2/v2
	//     3. v1->group2/v2
	//     4. v1->v2
	// b) One of these is an exact match and it is already returned earlier.
	fromMatches := []Rule{}
	toMatches := []Rule{}
	idxFrom := strings.IndexRune(rule.FromVersion, '/')
	idxTo := strings.IndexRune(rule.ToVersion, '/')
	for _, pathKey := range pathKeys {
		cc := 0
		if idxFrom >= 0 && pathKey.FromVersion == rule.FromVersion {
			fromMatches = append(fromMatches, pathKey)
			cc++
		}
		if idxTo >= 0 && pathKey.ToVersion == rule.ToVersion {
			toMatches = append(toMatches, pathKey)
			cc++
		}
		if cc == 2 {
			// Full equality of full versions is the most priority
			// E.g., input path is v1->group2/v2 and the cache has group1/v1->group2/v2.
			return c.PathsCache[pathKey]
		}
	}
	// toMatches should contain a path with full toVersion.
	// E.g., input path is group1/v1->v2 and the cache has v1->group2/v2.
	if len(toMatches) > 0 {
		return c.PathsCache[toMatches[0]]
	}
	// fromMatches should contain a path with full fromVersion.
	// E.g., input path is v1->v2 and the cache has group1/v1->v2.
	if len(fromMatches) > 0 {
		return c.PathsCache[fromMatches[0]]
	}
	// A fallback. Honestly, it should not happen.
	return c.PathsCache[pathKeys[0]]
}

// RulesWithFromVersion returns all rules in PathsCache that has similar FromVersion as the input rule.
func (c Chain) RulesWithSimilarFromVersion(rule Rule) []Rule {
	rules := []Rule{}
	for cachedPath := range c.PathsCache {
		if VersionsMatched(cachedPath.FromVersion, rule.FromVersion) {
			rules = append(rules, cachedPath)
		}
	}
	return rules
}

// NextRules finds all base paths in BaseFromToIndex that starts from an input version.
func (c Chain) NextRules(fromVer string) []Rule {
	rules := []Rule{}
	shortVer := string_helper.TrimGroup(fromVer)
	for k := range c.BaseFromToIndex {
		//
		if k == fromVer {
			for toVer := range c.BaseFromToIndex[k] {
				rules = append(rules, Rule{
					FromVersion: k,
					ToVersion:   toVer,
				})
			}
			continue
		}

		idxFrom := strings.Index(k, shortVer)
		if idxFrom == -1 {
			continue
		}
		for toVer := range c.BaseFromToIndex[k] {
			rules = append(rules, Rule{
				FromVersion: k,
				ToVersion:   toVer,
			})
		}
	}

	return rules
}

// HasTargetVersion returns true if there is a base path leading to an input version.
func (c Chain) HasTargetVersion(target string) bool {
	for fromVer := range c.BaseFromToIndex {
		for toVer := range c.BaseFromToIndex[fromVer] {
			if VersionsMatched(target, toVer) {
				return true
			}
		}
	}
	return false
}

// VersionsMatched when:
// - ver0 equals to ver1
// - ver0 is short and ver1 is full and short ver1 is equals to ver0
// - ver0 is full and ver1 is short and short ver0 is equals to ver1
// Note that ver0 and ver1 with different groups are not matched (e.g. stable/v1 and unstable/v1)
func VersionsMatched(v0, v1 string) bool {
	if v0 == v1 {
		return true
	}
	idx0 := strings.IndexRune(v0, '/')
	idx1 := strings.IndexRune(v1, '/')
	if idx0 == -1 && idx1 >= 0 {
		return v0 == v1[idx1+1:]
	}
	if idx0 >= 0 && idx1 == -1 {
		return v0[idx0+1:] == v1
	}
	return false
}
