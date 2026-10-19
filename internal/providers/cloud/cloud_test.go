// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cloud

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3control/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

var ctx = context.Background()

// ---- AWS fakes

type fakeSTS struct{}

func (fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{Account: aws.String("123456789012")}, nil
}

type fakeS3Control struct {
	cfg *s3types.PublicAccessBlockConfiguration
	err error
}

func (f fakeS3Control) GetPublicAccessBlock(context.Context, *s3control.GetPublicAccessBlockInput, ...func(*s3control.Options)) (*s3control.GetPublicAccessBlockOutput, error) {
	return &s3control.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: f.cfg}, f.err
}

func TestS3AccountPublicAccessBlock(t *testing.T) {
	all := &s3types.PublicAccessBlockConfiguration{BlockPublicAcls: aws.Bool(true), IgnorePublicAcls: aws.Bool(true), BlockPublicPolicy: aws.Bool(true), RestrictPublicBuckets: aws.Bool(true)}
	partial := &s3types.PublicAccessBlockConfiguration{BlockPublicAcls: aws.Bool(true), IgnorePublicAcls: aws.Bool(true), BlockPublicPolicy: aws.Bool(true), RestrictPublicBuckets: aws.Bool(false)}
	notFound := &smithy.GenericAPIError{Code: "NoSuchPublicAccessBlockConfiguration"}

	cases := []struct {
		name      string
		api       fakeS3Control
		compliant bool
		detail    string
		wantErr   bool
	}{
		{"all four on", fakeS3Control{cfg: all}, true, "", false},
		{"one off", fakeS3Control{cfg: partial}, false, "RestrictPublicBuckets", false},
		{"not configured", fakeS3Control{err: notFound}, false, "no account-level", false},
		{"api failure", fakeS3Control{err: errors.New("denied")}, false, "", true},
	}
	for _, c := range cases {
		got, err := checkS3AccountPublicAccessBlock(ctx, fakeSTS{}, c.api)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v", c.name, err)
			continue
		}
		if c.wantErr {
			continue
		}
		if got[0].Compliant != c.compliant || !strings.Contains(got[0].Detail, c.detail) || got[0].Resource != "123456789012" {
			t.Errorf("%s: got %+v", c.name, got[0])
		}
	}
}

type fakeIAM struct {
	mfa    int32
	pwLen  *int32
	pwErr  error
	sumErr error
}

func (f fakeIAM) GetAccountSummary(context.Context, *iam.GetAccountSummaryInput, ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error) {
	return &iam.GetAccountSummaryOutput{SummaryMap: map[string]int32{"AccountMFAEnabled": f.mfa}}, f.sumErr
}
func (f fakeIAM) GetAccountPasswordPolicy(context.Context, *iam.GetAccountPasswordPolicyInput, ...func(*iam.Options)) (*iam.GetAccountPasswordPolicyOutput, error) {
	if f.pwErr != nil {
		return nil, f.pwErr
	}
	return &iam.GetAccountPasswordPolicyOutput{PasswordPolicy: &iamtypes.PasswordPolicy{MinimumPasswordLength: f.pwLen}}, nil
}

func TestRootMFAAndPasswordPolicy(t *testing.T) {
	if f, _ := checkRootMFA(ctx, fakeIAM{mfa: 1}); !f[0].Compliant {
		t.Error("MFA enabled should pass")
	}
	if f, _ := checkRootMFA(ctx, fakeIAM{mfa: 0}); f[0].Compliant {
		t.Error("MFA disabled should fail")
	}
	if _, err := checkRootMFA(ctx, fakeIAM{sumErr: errors.New("x")}); err == nil {
		t.Error("api error must propagate")
	}
	if f, _ := checkPasswordPolicy(ctx, fakeIAM{pwLen: aws.Int32(14)}); !f[0].Compliant {
		t.Error("14 should pass")
	}
	if f, _ := checkPasswordPolicy(ctx, fakeIAM{pwLen: aws.Int32(8)}); f[0].Compliant || !strings.Contains(f[0].Detail, "8") {
		t.Errorf("8 should fail: %+v", f[0])
	}
	if f, _ := checkPasswordPolicy(ctx, fakeIAM{pwErr: &smithy.GenericAPIError{Code: "NoSuchEntity"}}); f[0].Compliant {
		t.Error("missing policy should fail")
	}
}

type fakeTrail struct {
	trails  []cttypes.Trail
	logging map[string]bool
}

func (f fakeTrail) DescribeTrails(context.Context, *cloudtrail.DescribeTrailsInput, ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
	return &cloudtrail.DescribeTrailsOutput{TrailList: f.trails}, nil
}
func (f fakeTrail) GetTrailStatus(_ context.Context, in *cloudtrail.GetTrailStatusInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error) {
	return &cloudtrail.GetTrailStatusOutput{IsLogging: aws.Bool(f.logging[*in.Name])}, nil
}

func TestCloudTrail(t *testing.T) {
	multi := cttypes.Trail{Name: aws.String("org"), TrailARN: aws.String("arn:org"), IsMultiRegionTrail: aws.Bool(true)}
	single := cttypes.Trail{Name: aws.String("one"), TrailARN: aws.String("arn:one"), IsMultiRegionTrail: aws.Bool(false)}
	if f, _ := checkCloudTrail(ctx, fakeTrail{trails: []cttypes.Trail{single, multi}, logging: map[string]bool{"arn:org": true}}); !f[0].Compliant || f[0].Resource != "org" {
		t.Errorf("multi-region logging trail should pass: %+v", f)
	}
	if f, _ := checkCloudTrail(ctx, fakeTrail{trails: []cttypes.Trail{multi}, logging: map[string]bool{"arn:org": false}}); f[0].Compliant {
		t.Error("stopped trail must fail")
	}
	if f, _ := checkCloudTrail(ctx, fakeTrail{trails: []cttypes.Trail{single}, logging: map[string]bool{"arn:one": true}}); f[0].Compliant {
		t.Error("single-region trail must fail")
	}
}

// ---- Azure / GCP fakes

type fakeStorage struct {
	accts []*armstorage.Account
	err   error
}

func (f fakeStorage) listAccounts(context.Context) ([]*armstorage.Account, error) {
	return f.accts, f.err
}

func acct(name string, https, blob *bool) *armstorage.Account {
	return &armstorage.Account{Name: &name, Properties: &armstorage.AccountProperties{EnableHTTPSTrafficOnly: https, AllowBlobPublicAccess: blob}}
}

func TestAzureStorage(t *testing.T) {
	l := fakeStorage{accts: []*armstorage.Account{
		acct("good", aws.Bool(true), aws.Bool(false)),
		acct("nohttps", aws.Bool(false), aws.Bool(false)),
		acct("public", nil, aws.Bool(true)),
		acct("unset", nil, nil),
	}}
	https, _ := checkStorageSecureTransfer(ctx, l)
	want := map[string]bool{"good": true, "nohttps": false, "public": true, "unset": true} // unset == Azure default (true)
	for _, f := range https {
		if f.Compliant != want[f.Resource] {
			t.Errorf("https %s = %v", f.Resource, f.Compliant)
		}
	}
	blob, _ := checkStorageNoPublicBlob(ctx, l)
	want = map[string]bool{"good": true, "nohttps": true, "public": false, "unset": false}
	for _, f := range blob {
		if f.Compliant != want[f.Resource] {
			t.Errorf("blob %s = %v", f.Resource, f.Compliant)
		}
	}
	if _, err := checkStorageSecureTransfer(ctx, fakeStorage{err: errors.New("403")}); err == nil {
		t.Error("list error must propagate")
	}
}

type fakeBuckets []*storage.BucketAttrs

func (f fakeBuckets) listBuckets(context.Context) ([]*storage.BucketAttrs, error) { return f, nil }

func TestGCPBuckets(t *testing.T) {
	l := fakeBuckets{
		{Name: "ok", UniformBucketLevelAccess: storage.UniformBucketLevelAccess{Enabled: true}, PublicAccessPrevention: storage.PublicAccessPreventionEnforced},
		{Name: "legacy", PublicAccessPrevention: storage.PublicAccessPreventionInherited},
	}
	u, _ := checkBucketUniformAccess(ctx, l)
	p, _ := checkBucketPublicAccessPrevention(ctx, l)
	if !u[0].Compliant || u[1].Compliant || !p[0].Compliant || p[1].Compliant {
		t.Fatalf("unexpected: %+v %+v", u, p)
	}
}

// ---- registry / helpers

func TestRegistryAndTerraform(t *testing.T) {
	ids := IDs()
	if len(ids) != 8 {
		t.Fatalf("expected 8 checks, got %v", ids)
	}
	for _, id := range ids {
		c, ok := Get(id)
		if !ok || c.Run == nil || c.Terraform == nil || c.Title == "" || !strings.HasPrefix(id, c.Provider+".") {
			t.Errorf("bad registration for %s", id)
		}
		if tf := c.Terraform(Finding{Resource: "my-res.1"}); tf == "" {
			t.Errorf("%s: empty terraform", id)
		}
	}
	if _, ok := Get("nope"); ok {
		t.Error("unknown id must not resolve")
	}
	c, _ := Get("gcp.storage.uniform_access")
	if tf := c.Terraform(Finding{Resource: "my-bucket.1"}); !strings.Contains(tf, `"my_bucket_1"`) || !strings.Contains(tf, "uniform_bucket_level_access = true") {
		t.Errorf("terraform: %s", tf)
	}
}

func TestSummarizeAndTfName(t *testing.T) {
	if ok, _ := Summarize(nil); ok {
		t.Error("no findings must not be compliant")
	}
	ok, msg := Summarize([]Finding{{Resource: "a", Compliant: true}, {Resource: "b", Detail: "bad"}})
	if ok || !strings.Contains(msg, "b (bad)") || strings.Contains(msg, "a;") || strings.HasPrefix(msg, "a") {
		t.Errorf("summarize: %v %q", ok, msg)
	}
	if tfName("9-a.b") != "r_9_a_b" || tfName("") != "r_" {
		t.Errorf("tfName: %q %q", tfName("9-a.b"), tfName(""))
	}
}
