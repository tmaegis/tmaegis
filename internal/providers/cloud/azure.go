// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cloud

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

// storageLister hides the SDK pager so the check logic is testable without Azure.
type storageLister interface {
	listAccounts(ctx context.Context) ([]*armstorage.Account, error)
}

type armStorageLister struct{ client *armstorage.AccountsClient }

func (l armStorageLister) listAccounts(ctx context.Context) ([]*armstorage.Account, error) {
	var out []*armstorage.Account
	pager := l.client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Value...)
	}
	return out, nil
}

func newStorageLister() (storageLister, error) {
	sub := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if sub == "" {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID is not set")
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	client, err := armstorage.NewAccountsClient(sub, cred, nil)
	if err != nil {
		return nil, err
	}
	return armStorageLister{client}, nil
}

func accountName(a *armstorage.Account) string {
	if a != nil && a.Name != nil {
		return *a.Name
	}
	return "unknown"
}

// checkStorageSecureTransfer requires HTTPS-only access. Azure defaults this to true,
// so an unset value is compliant.
func checkStorageSecureTransfer(ctx context.Context, l storageLister) ([]Finding, error) {
	accts, err := l.listAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing storage accounts: %w", err)
	}
	var out []Finding
	for _, a := range accts {
		f := Finding{Resource: accountName(a), Compliant: true}
		if a.Properties != nil && a.Properties.EnableHTTPSTrafficOnly != nil && !*a.Properties.EnableHTTPSTrafficOnly {
			f.Compliant, f.Detail = false, "secure transfer (HTTPS only) is disabled"
		}
		out = append(out, f)
	}
	return out, nil
}

// checkStorageNoPublicBlob requires anonymous blob access to be explicitly disallowed.
func checkStorageNoPublicBlob(ctx context.Context, l storageLister) ([]Finding, error) {
	accts, err := l.listAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing storage accounts: %w", err)
	}
	var out []Finding
	for _, a := range accts {
		f := Finding{Resource: accountName(a)}
		switch {
		case a.Properties == nil || a.Properties.AllowBlobPublicAccess == nil:
			f.Detail = "anonymous blob access is not explicitly disabled"
		case *a.Properties.AllowBlobPublicAccess:
			f.Detail = "anonymous blob access is allowed"
		default:
			f.Compliant = true
		}
		out = append(out, f)
	}
	return out, nil
}

func init() {
	run := func(fn func(context.Context, storageLister) ([]Finding, error)) func(context.Context) ([]Finding, error) {
		return func(ctx context.Context) ([]Finding, error) {
			l, err := newStorageLister()
			if err != nil {
				return nil, err
			}
			return fn(ctx, l)
		}
	}
	register(&Check{
		ID: "azure.storage.secure_transfer", Provider: "azure", Title: "Storage accounts require secure transfer",
		Run: run(checkStorageSecureTransfer),
		Terraform: func(f Finding) string {
			return fmt.Sprintf("# Add to the existing azurerm_storage_account resource for %q:\nresource \"azurerm_storage_account\" %q {\n  # ...existing arguments...\n  https_traffic_only_enabled = true\n  min_tls_version            = \"TLS1_2\"\n}\n", f.Resource, tfName(f.Resource))
		},
	})
	register(&Check{
		ID: "azure.storage.no_public_blob", Provider: "azure", Title: "Storage accounts disallow anonymous blob access",
		Run: run(checkStorageNoPublicBlob),
		Terraform: func(f Finding) string {
			return fmt.Sprintf("# Add to the existing azurerm_storage_account resource for %q:\nresource \"azurerm_storage_account\" %q {\n  # ...existing arguments...\n  allow_nested_items_to_be_public = false\n}\n", f.Resource, tfName(f.Resource))
		},
	})
}
