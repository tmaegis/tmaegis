// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cloud

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

const minPasswordLength = 14

type s3controlAPI interface {
	GetPublicAccessBlock(context.Context, *s3control.GetPublicAccessBlockInput, ...func(*s3control.Options)) (*s3control.GetPublicAccessBlockOutput, error)
}
type stsAPI interface {
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}
type iamAPI interface {
	GetAccountSummary(context.Context, *iam.GetAccountSummaryInput, ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error)
	GetAccountPasswordPolicy(context.Context, *iam.GetAccountPasswordPolicyInput, ...func(*iam.Options)) (*iam.GetAccountPasswordPolicyOutput, error)
}
type cloudtrailAPI interface {
	DescribeTrails(context.Context, *cloudtrail.DescribeTrailsInput, ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error)
	GetTrailStatus(context.Context, *cloudtrail.GetTrailStatusInput, ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error)
}

func hasErrorCode(err error, code string) bool {
	var ae smithy.APIError
	return errors.As(err, &ae) && ae.ErrorCode() == code
}

func ptrTrue(b *bool) bool { return b != nil && *b }

// checkS3AccountPublicAccessBlock requires all four account-level S3 Block Public Access settings.
func checkS3AccountPublicAccessBlock(ctx context.Context, sc stsAPI, s3c s3controlAPI) ([]Finding, error) {
	id, err := sc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("sts:GetCallerIdentity: %w", err)
	}
	acct := ""
	if id.Account != nil {
		acct = *id.Account
	}
	out, err := s3c.GetPublicAccessBlock(ctx, &s3control.GetPublicAccessBlockInput{AccountId: id.Account})
	if err != nil {
		if hasErrorCode(err, "NoSuchPublicAccessBlockConfiguration") {
			return []Finding{{Resource: acct, Detail: "no account-level public access block configured"}}, nil
		}
		return nil, fmt.Errorf("s3control:GetPublicAccessBlock: %w", err)
	}
	c := out.PublicAccessBlockConfiguration
	if c == nil {
		return []Finding{{Resource: acct, Detail: "empty configuration"}}, nil
	}
	var missing []string
	for name, v := range map[string]*bool{
		"BlockPublicAcls": c.BlockPublicAcls, "IgnorePublicAcls": c.IgnorePublicAcls,
		"BlockPublicPolicy": c.BlockPublicPolicy, "RestrictPublicBuckets": c.RestrictPublicBuckets,
	} {
		if !ptrTrue(v) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return []Finding{{Resource: acct, Detail: fmt.Sprintf("not enabled: %v", sortedStrings(missing))}}, nil
	}
	return []Finding{{Resource: acct, Compliant: true}}, nil
}

func checkRootMFA(ctx context.Context, api iamAPI) ([]Finding, error) {
	out, err := api.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		return nil, fmt.Errorf("iam:GetAccountSummary: %w", err)
	}
	if out.SummaryMap["AccountMFAEnabled"] == 1 {
		return []Finding{{Resource: "root", Compliant: true}}, nil
	}
	return []Finding{{Resource: "root", Detail: "MFA is not enabled for the root user"}}, nil
}

func checkPasswordPolicy(ctx context.Context, api iamAPI) ([]Finding, error) {
	out, err := api.GetAccountPasswordPolicy(ctx, &iam.GetAccountPasswordPolicyInput{})
	if err != nil {
		if hasErrorCode(err, "NoSuchEntity") {
			return []Finding{{Resource: "account", Detail: "no password policy set"}}, nil
		}
		return nil, fmt.Errorf("iam:GetAccountPasswordPolicy: %w", err)
	}
	if p := out.PasswordPolicy; p != nil && p.MinimumPasswordLength != nil && int(*p.MinimumPasswordLength) >= minPasswordLength {
		return []Finding{{Resource: "account", Compliant: true}}, nil
	}
	got := "unset"
	if out.PasswordPolicy != nil && out.PasswordPolicy.MinimumPasswordLength != nil {
		got = fmt.Sprint(*out.PasswordPolicy.MinimumPasswordLength)
	}
	return []Finding{{Resource: "account", Detail: fmt.Sprintf("minimum password length is %s, need >= %d", got, minPasswordLength)}}, nil
}

// checkCloudTrail requires at least one multi-region trail that is actively logging.
func checkCloudTrail(ctx context.Context, api cloudtrailAPI) ([]Finding, error) {
	out, err := api.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{})
	if err != nil {
		return nil, fmt.Errorf("cloudtrail:DescribeTrails: %w", err)
	}
	for _, t := range out.TrailList {
		if !ptrTrue(t.IsMultiRegionTrail) || t.TrailARN == nil {
			continue
		}
		st, err := api.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{Name: t.TrailARN})
		if err != nil {
			return nil, fmt.Errorf("cloudtrail:GetTrailStatus: %w", err)
		}
		if ptrTrue(st.IsLogging) {
			name := *t.TrailARN
			if t.Name != nil {
				name = *t.Name
			}
			return []Finding{{Resource: name, Compliant: true}}, nil
		}
	}
	return []Finding{{Resource: "account", Detail: "no multi-region trail with logging enabled"}}, nil
}

func sortedStrings(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func init() {
	loadCfg := func(ctx context.Context) (awsCfg, error) {
		c, err := config.LoadDefaultConfig(ctx)
		return awsCfg{c}, err
	}
	register(&Check{
		ID: "aws.s3.account_public_access_block", Provider: "aws",
		Title: "S3 Block Public Access is enabled at account level",
		Run: func(ctx context.Context) ([]Finding, error) {
			c, err := loadCfg(ctx)
			if err != nil {
				return nil, err
			}
			return checkS3AccountPublicAccessBlock(ctx, sts.NewFromConfig(c.Config), s3control.NewFromConfig(c.Config))
		},
		Terraform: func(Finding) string {
			return `resource "aws_s3_account_public_access_block" "this" {
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
`
		},
	})
	register(&Check{
		ID: "aws.iam.root_mfa", Provider: "aws", Title: "MFA is enabled for the root user",
		Run: func(ctx context.Context) ([]Finding, error) {
			c, err := loadCfg(ctx)
			if err != nil {
				return nil, err
			}
			return checkRootMFA(ctx, iam.NewFromConfig(c.Config))
		},
		Terraform: func(Finding) string {
			return "# Root MFA cannot be managed with Terraform.\n# Sign in as the root user and register a hardware or virtual MFA device (IAM > Security credentials).\n"
		},
	})
	register(&Check{
		ID: "aws.iam.password_policy", Provider: "aws", Title: "IAM password policy requires 14+ characters",
		Run: func(ctx context.Context) ([]Finding, error) {
			c, err := loadCfg(ctx)
			if err != nil {
				return nil, err
			}
			return checkPasswordPolicy(ctx, iam.NewFromConfig(c.Config))
		},
		Terraform: func(Finding) string {
			return `resource "aws_iam_account_password_policy" "this" {
  minimum_password_length = 14
  require_lowercase_characters = true
  require_uppercase_characters = true
  require_numbers              = true
  require_symbols              = true
  password_reuse_prevention    = 24
  max_password_age             = 90
}
`
		},
	})
	register(&Check{
		ID: "aws.cloudtrail.multi_region", Provider: "aws", Title: "A multi-region CloudTrail is logging",
		Run: func(ctx context.Context) ([]Finding, error) {
			c, err := loadCfg(ctx)
			if err != nil {
				return nil, err
			}
			return checkCloudTrail(ctx, cloudtrail.NewFromConfig(c.Config))
		},
		Terraform: func(Finding) string {
			return `resource "aws_cloudtrail" "main" {
  name                          = "org-trail"
  s3_bucket_name                = var.cloudtrail_bucket # existing bucket with the CloudTrail bucket policy
  is_multi_region_trail         = true
  include_global_service_events = true
  enable_log_file_validation    = true
}
`
		},
	})
}
