// Package operator watches PBOM custom resources and refreshes the application service.
package operator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/discovery"
	"github.com/ravibagri5/platform-bom/internal/service"
)

// DisplayNameAnnotation carries a resource's display name.
const DisplayNameAnnotation = api.DisplayNameAnnotation

var resources = []schema.GroupVersionResource{
	{Group: "pbom.dev", Version: "v1alpha1", Resource: "platforms"},
	{Group: "pbom.dev", Version: "v1alpha1", Resource: "platformreleases"},
	{Group: "pbom.dev", Version: "v1alpha1", Resource: "components"},
	{Group: "pbom.dev", Version: "v1alpha1", Resource: "inventories"},
}

// Options select the namespace and Platform served by this PBOM instance.
type Options struct {
	Namespace    string
	PlatformName string
	// Context is the kubeconfig context to watch; empty uses the current one, or the pod's service account.
	Context string
	Service service.Options
}

// Run watches namespaced PBOM resources until ctx is cancelled.
func Run(ctx context.Context, opts Options, swap func(*service.Service)) error {
	config, err := discovery.RESTConfig("", opts.Context)
	if err != nil {
		return fmt.Errorf("load Kubernetes config: %w", err)
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create Kubernetes client: %w", err)
	}
	return runWithClient(ctx, client, opts, swap)
}

func runWithClient(ctx context.Context, client dynamic.Interface, opts Options, swap func(*service.Service)) error {
	if opts.Namespace == "" {
		return errors.New("CRD source requires a namespace")
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, 0, opts.Namespace, nil)
	informers := make([]cache.SharedIndexInformer, 0, len(resources))
	changes := make(chan struct{}, 1)
	changed := func(any) {
		select {
		case changes <- struct{}{}:
		default:
		}
	}
	for _, gvr := range resources {
		informer := factory.ForResource(gvr)
		if _, err := informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    changed,
			UpdateFunc: func(any, any) { changed(nil) },
			DeleteFunc: changed,
		}); err != nil {
			return fmt.Errorf("watch %s: %w", gvr.Resource, err)
		}
		informers = append(informers, informer.Informer())
	}
	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informerSyncs(informers)...) {
		return ctx.Err()
	}

	build := func() error {
		objects := make([][]*unstructured.Unstructured, len(informers))
		for i, informer := range informers {
			for _, item := range informer.GetStore().List() {
				obj, ok := item.(*unstructured.Unstructured)
				if !ok {
					return fmt.Errorf("unexpected %s object %T", resources[i].Resource, item)
				}
				objects[i] = append(objects[i], obj)
			}
		}
		config, err := resourcesFromObjects(objects[0], objects[1], objects[2], objects[3], opts.PlatformName)
		if err != nil {
			return err
		}
		svc, err := service.NewFromResources(config, opts.Service)
		if err != nil {
			return err
		}
		swap(svc)
		slog.Info("reconciled PBOM resources", "platform", svc.Platform.Metadata.Name)
		return nil
	}
	if err := build(); err != nil {
		slog.Warn("PBOM resources are not ready; waiting for a valid configuration", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-changes:
			if err := build(); err != nil {
				slog.Warn("PBOM resources are invalid; keeping the previous configuration", "error", err)
			}
		}
	}
}

func informerSyncs(informers []cache.SharedIndexInformer) []cache.InformerSynced {
	syncs := make([]cache.InformerSynced, 0, len(informers))
	for _, informer := range informers {
		syncs = append(syncs, informer.HasSynced)
	}
	return syncs
}

func resourcesFromObjects(platforms, releases, components, inventories []*unstructured.Unstructured, platformName string) (service.Resources, error) {
	var selected *unstructured.Unstructured
	for _, obj := range platforms {
		if platformName != "" && obj.GetName() == platformName {
			selected = obj
			break
		}
		if platformName == "" {
			if selected != nil {
				return service.Resources{}, errors.New("multiple Platform resources found; set --platform-name")
			}
			selected = obj
		}
	}
	if selected == nil {
		if platformName != "" {
			return service.Resources{}, fmt.Errorf("platform %q not found", platformName)
		}
		return service.Resources{}, errors.New("no Platform resource found")
	}

	platform := &api.Platform{TypeMeta: typeMeta(selected, api.KindPlatform), Metadata: metadata(selected)}
	if err := decodeSpec(selected, &platform.Spec); err != nil {
		return service.Resources{}, fmt.Errorf("platform %q: %w", selected.GetName(), err)
	}
	result := service.Resources{Platform: platform}
	for _, obj := range releases {
		release := api.PlatformRelease{TypeMeta: typeMeta(obj, api.KindRelease), Metadata: metadata(obj)}
		if err := decodeSpec(obj, &release.Spec); err != nil {
			return service.Resources{}, fmt.Errorf("release %q: %w", obj.GetName(), err)
		}
		result.Releases = append(result.Releases, release)
	}
	for _, obj := range components {
		component := &api.Component{TypeMeta: typeMeta(obj, api.KindComponent), Metadata: metadata(obj)}
		if err := decodeSpec(obj, &component.Spec); err != nil {
			return service.Resources{}, fmt.Errorf("component %q: %w", obj.GetName(), err)
		}
		result.Components = append(result.Components, component)
	}
	for _, obj := range inventories {
		inv, err := decodeInventory(obj)
		if err != nil {
			return service.Resources{}, fmt.Errorf("inventory %q: %w", obj.GetName(), err)
		}
		result.Inventories = append(result.Inventories, inv)
	}
	return result, nil
}

// decodeInventory reads an Inventory, whose fields are top-level like the
// output of pbom discover -o yaml rather than under spec.
func decodeInventory(obj *unstructured.Unstructured) (*api.Inventory, error) {
	fields := make(map[string]any, len(obj.Object))
	for k, v := range obj.Object {
		if k != "metadata" {
			fields[k] = v
		}
	}
	data, err := yaml.Marshal(fields)
	if err != nil {
		return nil, err
	}
	inv := &api.Inventory{}
	if err := yaml.Unmarshal(data, inv); err != nil {
		return nil, err
	}
	inv.TypeMeta = typeMeta(obj, api.KindInventory)
	inv.Metadata = metadata(obj)
	return inv, nil
}

func decodeSpec(obj *unstructured.Unstructured, target any) error {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return err
	}
	if !found {
		return errors.New("spec is required")
	}
	data, err := yaml.Marshal(spec)
	if err != nil {
		return err
	}
	return yaml.UnmarshalStrict(data, target)
}

func typeMeta(obj *unstructured.Unstructured, kind string) api.TypeMeta {
	version := obj.GetAPIVersion()
	if version == "" {
		version = api.APIVersion
	}
	return api.TypeMeta{APIVersion: version, Kind: kind}
}

func metadata(obj *unstructured.Unstructured) api.Metadata {
	return api.Metadata{
		Name:        obj.GetName(),
		DisplayName: obj.GetAnnotations()[DisplayNameAnnotation],
		Labels:      obj.GetLabels(),
	}
}
