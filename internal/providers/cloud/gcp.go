// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cloud

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type bucketLister interface {
	listBuckets(ctx context.Context) ([]*storage.BucketAttrs, error)
}

type gcsLister struct {
	client  *storage.Client
	project string
}

func (l gcsLister) listBuckets(ctx context.Context) ([]*storage.BucketAttrs, error) {
	var out []*storage.BucketAttrs
	it := l.client.Buckets(ctx, l.project)
	for {
		b, err := it.Next()
		if err == iterator.Done {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
}

func newBucketLister(ctx context.Context) (bucketLister, error) {
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "" {
		return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT is not set")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return gcsLister{client, project}, nil
}

func checkBucketUniformAccess(ctx context.Context, l bucketLister) ([]Finding, error) {
	buckets, err := l.listBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing buckets: %w", err)
	}
	var out []Finding
	for _, b := range buckets {
		f := Finding{Resource: b.Name, Compliant: b.UniformBucketLevelAccess.Enabled}
		if !f.Compliant {
			f.Detail = "uniform bucket-level access is disabled (legacy ACLs in effect)"
		}
		out = append(out, f)
	}
	return out, nil
}

func checkBucketPublicAccessPrevention(ctx context.Context, l bucketLister) ([]Finding, error) {
	buckets, err := l.listBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing buckets: %w", err)
	}
	var out []Finding
	for _, b := range buckets {
		f := Finding{Resource: b.Name, Compliant: b.PublicAccessPrevention == storage.PublicAccessPreventionEnforced}
		if !f.Compliant {
			f.Detail = "public access prevention is not enforced"
		}
		out = append(out, f)
	}
	return out, nil
}

func init() {
	run := func(fn func(context.Context, bucketLister) ([]Finding, error)) func(context.Context) ([]Finding, error) {
		return func(ctx context.Context) ([]Finding, error) {
			l, err := newBucketLister(ctx)
			if err != nil {
				return nil, err
			}
			return fn(ctx, l)
		}
	}
	register(&Check{
		ID: "gcp.storage.uniform_access", Provider: "gcp", Title: "Buckets use uniform bucket-level access",
		Run: run(checkBucketUniformAccess),
		Terraform: func(f Finding) string {
			return fmt.Sprintf("# Add to the existing google_storage_bucket resource for %q:\nresource \"google_storage_bucket\" %q {\n  # ...existing arguments...\n  uniform_bucket_level_access = true\n}\n", f.Resource, tfName(f.Resource))
		},
	})
	register(&Check{
		ID: "gcp.storage.public_access_prevention", Provider: "gcp", Title: "Buckets enforce public access prevention",
		Run: run(checkBucketPublicAccessPrevention),
		Terraform: func(f Finding) string {
			return fmt.Sprintf("# Add to the existing google_storage_bucket resource for %q:\nresource \"google_storage_bucket\" %q {\n  # ...existing arguments...\n  public_access_prevention = \"enforced\"\n}\n", f.Resource, tfName(f.Resource))
		},
	})
}
