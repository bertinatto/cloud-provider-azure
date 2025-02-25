// /*
// Copyright The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

// Package generator
package generator

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/controller-tools/pkg/genall"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

func generateTest(ctx *genall.GenerationContext, root *loader.Package, headerText string) error {
	var importList = make(map[string]map[string]struct{})
	var codeSnips []*bytes.Buffer
	err := markers.EachType(ctx.Collector, root, func(typeInfo *markers.TypeInfo) {
		if typeInfo := typeInfo.Markers.Get(clientGenMarker.Name); typeInfo != nil {
			markerConf := typeInfo.(ClientGenConfig)

			var outContent bytes.Buffer

			importList["context"] = make(map[string]struct{})
			importList["testing"] = make(map[string]struct{})
			importList["time"] = make(map[string]struct{})
			importList["sigs.k8s.io/cloud-provider-azure/pkg/azclient/recording"] = make(map[string]struct{})
			importList["github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"] = make(map[string]struct{})
			importList["github.com/Azure/azure-sdk-for-go/sdk/azcore"] = make(map[string]struct{})
			importList["github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"] = make(map[string]struct{})
			importList["github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"] = make(map[string]struct{})
			importList["github.com/Azure/azure-sdk-for-go/sdk/azcore/to"] = make(map[string]struct{})
			importList["github.com/onsi/ginkgo/v2"] = map[string]struct{}{".": {}}
			importList["github.com/onsi/gomega"] = map[string]struct{}{".": {}}

			if err := TestSuiteTemplate.Execute(&outContent, markerConf); err != nil {
				root.AddError(err)
			}
			codeSnips = append(codeSnips, &outContent)
		}
	})
	if err != nil {
		root.AddError(err)
		return err
	}
	if len(codeSnips) <= 0 {
		return nil
	}

	outContent := new(bytes.Buffer)

	var importStatement bytes.Buffer
	importWriter := bufio.NewWriter(&importStatement)
	for packageName, alias := range importList {
		if len(alias) == 0 {
			if err := ImportTemplate.Execute(importWriter, &ImportStatement{Alias: "", Package: packageName}); err != nil {
				return err
			}
		}
		for item := range alias {
			if err := ImportTemplate.Execute(importWriter, &ImportStatement{Alias: item, Package: packageName}); err != nil {
				return err
			}
		}
	}
	importWriter.Flush()
	_, err = fmt.Fprintf(outContent, `
%[3]s
// Code generated by client-gen. DO NOT EDIT.
package %[1]s
import (
  %[2]s
)
`, root.Name, importStatement.String(), headerText)
	if err != nil {
		return err
	}

	for _, codeSnip := range codeSnips {
		if _, err := io.Copy(outContent, bufio.NewReader(codeSnip)); err != nil {
			return err
		}
	}

	rawBytes := outContent.Bytes()
	outputFile, err := ctx.Open(root, root.Name+"_suite_test.go")
	if err != nil {
		return err
	}
	defer outputFile.Close()
	n, err := outputFile.Write(rawBytes)
	if err != nil {
		return err
	}
	if n < len(rawBytes) {
		return io.ErrShortWrite
	}
	if err := os.MkdirAll(root.Name+"/testdata", 0755); err != nil {
		return err
	}
	return nil
}
