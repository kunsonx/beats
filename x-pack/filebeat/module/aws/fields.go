// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Code generated by beats/dev-tools/cmd/asset/asset.go - DO NOT EDIT.

package aws

import (
	"github.com/elastic/beats/v7/libbeat/asset"
)

func init() {
	if err := asset.SetFields("filebeat", "aws", asset.ModuleFieldsPri, AssetAws); err != nil {
		panic(err)
	}
}

// AssetAws returns asset data.
// This is the base64 encoded zlib format compressed contents of module/aws.
func AssetAws() string {
	return "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"
}