package discovery

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
)

// maxHelmReleaseSize bounds decompressed release payloads.
const maxHelmReleaseSize = 32 << 20

type helmRelease struct {
	Name  string `json:"name"`
	Chart struct {
		Metadata struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			AppVersion string `json:"appVersion"`
		} `json:"metadata"`
	} `json:"chart"`
}

// decodeHelmRelease decodes the "release" field of a Helm v3 storage Secret,
// which is base64 encoded, usually gzipped JSON.
func decodeHelmRelease(data []byte) (*helmRelease, error) {
	raw, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, err
	}
	var r io.Reader = bytes.NewReader(raw)
	if len(raw) > 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}
	rel := &helmRelease{}
	if err := json.NewDecoder(io.LimitReader(r, maxHelmReleaseSize)).Decode(rel); err != nil {
		return nil, err
	}
	return rel, nil
}
