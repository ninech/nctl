package api

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type WatchFunc func(list runtimeclient.ObjectList) error

// TODO: all-projects/all-namespaces options don't work as expected. The initial
// list call is correct but the watch call just happens in the current project.
// All namespaces is easy to solve but all projects would mean a goroutine/watch
// per project.
func (c *Client) watch(ctx context.Context, list runtimeclient.ObjectList, options ...ListOpt) error {
	// this is a bit awkward: we need to extract some functional options and
	// make sure we don't pass the watch option down to c.ListObjects as that
	// will cause infinite recursion. We do this by executing each opt and
	// checking the result.
	newOptions := []ListOpt{}
	var watchFunc WatchFunc
	var clientListOptions []runtimeclient.ListOption
	for _, opt := range options {
		opts := &ListOpts{}
		opt(opts)
		if !opts.watch {
			newOptions = append(newOptions, opt)
		}
		if opts.watchFunc != nil {
			watchFunc = opts.watchFunc
		}
		if opts.clientListOptions != nil {
			clientListOptions = opts.clientListOptions
		}
	}
	// do an initial list call, this is to get immediate output of the current
	// list if it has any items. After that updates will be streamed in by the
	// watch. If we would just call watch immediately, the list wouldn't be
	// properly formatted as the tab writer and sort logic would not have the
	// full list to work with.
	if err := c.ListObjects(ctx, list, newOptions...); err != nil {
		return err
	}
	items, err := itemsFromObjectList(list)
	if err != nil {
		return err
	}
	if items.Len() > 0 {
		if err := watchFunc(list); err != nil {
			return err
		}
	}

	wa, err := c.Watch(ctx, list, append(clientListOptions, &runtimeclient.ListOptions{
		Namespace: c.Project,
		Raw: &metav1.ListOptions{
			// in order to get around the initial list of items, we make a
			// previous list call and then set the resource version of the
			// returned list.
			ResourceVersion: list.GetResourceVersion(),
		},
	})...)
	if err != nil {
		return fmt.Errorf("watching resources: %w", err)
	}

	for {
		select {
		case res := <-wa.ResultChan():
			if res.Type == watch.Error || res.Type == "" {
				return fmt.Errorf("watching: %s", res.Object.GetObjectKind())
			}
			if err := meta.SetList(list, []runtime.Object{res.Object}); err != nil {
				return err
			}
			if err := watchFunc(list); err != nil {
				return err
			}
		case <-ctx.Done():
			wa.Stop()
			return nil
		}
	}
}
