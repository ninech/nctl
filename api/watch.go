package api

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
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
	// this is a bit awkward: we need to extract some functional options. We do
	// this by executing each opt and checking the result.
	var watchFunc WatchFunc
	var clientListOptions []runtimeclient.ListOption
	for _, opt := range options {
		opts := &ListOpts{}
		opt(opts)
		if opts.watchFunc != nil {
			watchFunc = opts.watchFunc
		}
		if opts.clientListOptions != nil {
			clientListOptions = opts.clientListOptions
		}
	}

	wa, err := c.Watch(ctx, list, append(clientListOptions, &runtimeclient.ListOptions{
		Namespace: c.Project,
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
