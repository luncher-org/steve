package converter

import (
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema/table"
	apiextv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/schemas"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	staticFields = map[string]schemas.Field{
		"apiVersion": {
			Type: "string",
		},
		"kind": {
			Type: "string",
		},
		"metadata": {
			Type:   "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
			Create: true,
			Update: true,
		},
	}
)

// addCustomResources uses the openAPISchema defined on CRDs to provide field definitions to previously discovered schemas.
// Note that this function does not create new schemas - it only adds details to resources already present in the schemas map.
func addCustomResources(crd apiextv1.CustomResourceDefinitionClient, schemas map[string]*types.APISchema) error {
	crds, err := crd.List(metav1.ListOptions{})
	if err != nil {
		return nil
	}

	for _, crd := range crds.Items {
		if crd.Status.AcceptedNames.Plural == "" {
			continue
		}

		group, kind := crd.Spec.Group, crd.Status.AcceptedNames.Kind

		for _, version := range crd.Spec.Versions {
			forVersion(group, kind, version, schemas)
		}
	}

	return nil
}

func forVersion(group, kind string, version v1.CustomResourceDefinitionVersion, schemasMap map[string]*types.APISchema) {
	var versionColumns []table.Column
	for _, col := range version.AdditionalPrinterColumns {
		versionColumns = append(versionColumns, table.Column{
			Name:   col.Name,
			Field:  col.JSONPath,
			Type:   col.Type,
			Format: col.Format,
		})
	}

	id := GVKToVersionedSchemaID(schema.GroupVersionKind{
		Group:   group,
		Version: version.Name,
		Kind:    kind,
	})

	schema := schemasMap[id]
	if schema == nil {
		return
	}
	attributes.MarkCRD(schema)
	if len(versionColumns) > 0 {
		attributes.SetColumns(schema, versionColumns)
	}
	if version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
		schema.Description = version.Schema.OpenAPIV3Schema.Description
	}
}
