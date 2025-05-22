package config

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

/**
 * Runtime configuration parameters. This is a simple flat KV storage.
 */

// Parameter is a runtime configuration parameter.
type Parameter struct {
	name          string
	description   string
	defaultValue  string
	onChange      func(oldValue string, newValue string) error
	forceDuration func(oldValue string, newValue string) time.Duration
}

// TemporalValue is a value for parameter that will expire in the future.
type TemporalValue struct {
	value  string
	expire time.Time
}

const CheckExpiredTemporalValuesPeriod = 15 * time.Second

// Config is a storage for all runtime parameters.
type Config struct {
	m      sync.Mutex
	params map[string]*Parameter
	values map[string]string
	// Errors from onChange handlers.
	errors map[string]error
	// Temporal values with expiration.
	temporalValues map[string]*TemporalValue
	expireTicker   *time.Ticker

	logger *log.Logger
}

func NewConfig(logger *log.Logger) *Config {
	return &Config{
		params:         make(map[string]*Parameter),
		values:         make(map[string]string),
		temporalValues: make(map[string]*TemporalValue),
		errors:         make(map[string]error),
		logger:         logger.With(slog.String("component", "runtimeConfig")),
	}
}

func (c *Config) Register(name string, description string, defaultValue string, onChange func(oldValue string, newValue string) error, forceDuration func(oldValue string, newValue string) time.Duration) {
	if c == nil {
		return
	}
	c.m.Lock()
	defer c.m.Unlock()

	if _, exists := c.params[name]; exists {
		return
	}
	c.params[name] = &Parameter{
		name:          name,
		defaultValue:  defaultValue,
		description:   description,
		onChange:      onChange,
		forceDuration: forceDuration,
	}
}

func (c *Config) List() []map[string]string {
	c.m.Lock()
	defer c.m.Unlock()

	res := make([]map[string]string, 0)
	for paramName, param := range c.params {
		paramValue := c.value(paramName)
		paramInfo := map[string]string{
			"name":        paramName,
			"description": param.description,
			"default":     param.defaultValue,
			"value":       paramValue,
		}
		if tempValue, ok := c.temporalValues[paramName]; ok {
			paramInfo["expireAt"] = tempValue.expire.Format(time.RFC3339)
		}
		lastError := c.errors[paramName]
		if lastError != nil {
			paramInfo["lastError"] = lastError.Error()
		}
		res = append(res, paramInfo)
	}
	sort.SliceStable(res, func(i, j int) bool {
		return res[i]["name"] < res[j]["name"]
	})
	return res
}

func (c *Config) String() string {
	b := new(strings.Builder)
	// Header
	fmt.Fprintf(b, "%-30s %-20s %-20s %-40s\n", "NAME", "VALUE", "EXPIRE AT", "DESCRIPTION")

	params := c.List()
	for _, param := range params {
		description := param["description"]
		if param["lastError"] != "" {
			description = "Error: " + param["lastError"]
		}
		fmt.Fprintf(b, "%-30s %-20s %-20s %-40s\n", param["name"], param["value"], param["expireAt"], description)
	}

	return b.String()
}

func (c *Config) Has(name string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	return c.has(name)
}

// IsValid validates runtime config parameter
func (c *Config) IsValid(name, value string) error {
	// maybe we should register it together with a parameter
	switch name {
	case "log.level":
		_, err := log.ParseLevel(value)
		if err != nil {
			return err
		}

	default:
		return nil
	}

	return nil
}

func (c *Config) has(name string) bool {
	_, registered := c.params[name]
	return registered
}

func (c *Config) LastError(name string) error {
	c.m.Lock()
	defer c.m.Unlock()

	err := c.errors[name]
	return err
}

// Set updates a value of the parameter by its name.
// Deletes a temporal value if set.
func (c *Config) Set(name string, value string) {
	forceDuration := c.callForceDuration(name, value)
	if forceDuration > 0 {
		c.SetTemporarily(name, value, forceDuration)
		return
	}

	c.m.Lock()
	oldValue := c.value(name)
	delete(c.temporalValues, name)
	c.values[name] = value
	c.m.Unlock()

	c.callOnChange(name, oldValue, value)
}

// Unset removes temporal value and value for parameter by its name.
func (c *Config) Unset(name string) {
	c.m.Lock()
	oldValue := c.value(name)
	delete(c.values, name)
	delete(c.temporalValues, name)
	newValue := c.value(name)
	c.m.Unlock()

	c.callOnChange(name, oldValue, newValue)
}

func (c *Config) Value(name string) string {
	c.m.Lock()
	defer c.m.Unlock()

	return c.value(name)
}

func (c *Config) value(name string) string {
	if !c.has(name) {
		return ""
	}

	if v, ok := c.temporalValues[name]; ok {
		return v.value
	}
	if v, ok := c.values[name]; ok {
		return v
	}
	return c.params[name].defaultValue
}

func (c *Config) SetTemporarily(name string, value string, duration time.Duration) {
	c.m.Lock()
	oldValue := c.value(name)
	delete(c.temporalValues, name)
	c.temporalValues[name] = &TemporalValue{
		value:  value,
		expire: time.Now().Add(duration),
	}
	newValue := c.value(name)
	// Start go routine to expire temporal values.
	if c.expireTicker == nil {
		c.expireTicker = time.NewTicker(CheckExpiredTemporalValuesPeriod)
		go func() {
			for {
				<-c.expireTicker.C
				c.expireOverrides()
			}
		}()
	}

	c.m.Unlock()

	c.callOnChange(name, oldValue, newValue)
}

// TODO accumulate changes and call onChange for all expired params (outside of locking, because callback can be long-lasted).
func (c *Config) expireOverrides() {
	if len(c.temporalValues) == 0 {
		return
	}

	now := time.Now()
	expires := make([][]string, 0)

	c.m.Lock()
	for name, temporalValue := range c.temporalValues {
		if temporalValue.expire.Before(now) {
			oldValue := c.value(name)
			delete(c.temporalValues, name)
			newValue := c.value(name)
			expires = append(expires, []string{name, oldValue, newValue})
		}
	}
	c.m.Unlock()

	for _, expire := range expires {
		name, oldValue, newValue := expire[0], expire[1], expire[2]
		c.logger.Debug("Parameter is expired", slog.String("parameter", name))
		c.callOnChange(name, oldValue, newValue)
	}
}

// callOnChange executes onChange handler if defined for the parameter.
func (c *Config) callOnChange(name string, oldValue string, newValue string) {
	if c.params[name].onChange == nil {
		return
	}
	err := c.params[name].onChange(oldValue, newValue)
	if err != nil {
		c.logger.Error("OnChange handler failed for parameter during value change values",
			slog.String("parameter", name), slog.String("old_value", oldValue), slog.String("new_value", newValue), log.Err(err))
	}
	c.m.Lock()
	delete(c.errors, name)
	c.errors[name] = err
	c.m.Unlock()
}

func (c *Config) callForceDuration(name string, newValue string) time.Duration {
	c.m.Lock()
	defer c.m.Unlock()
	param, has := c.params[name]
	if !has {
		return 0
	}
	if param.forceDuration == nil {
		return 0
	}

	oldValue := c.value(name)
	return param.forceDuration(oldValue, newValue)
}
