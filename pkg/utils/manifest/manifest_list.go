package manifest

import "github.com/flant/shell-operator/pkg/utils/manifest/releaseutil"

func GetManifestListFromYamlDocuments(rawManifests string) ([]Manifest, error) {
	var manifests []Manifest

	for _, doc := range releaseutil.SplitManifests(rawManifests) {
		m, err := NewManifestFromYaml(doc)
		if err != nil {
			return nil, err
		}

		if m.HasBasicFields() {
			manifests = append(manifests, m)
		}
	}

	return manifests, nil
}
