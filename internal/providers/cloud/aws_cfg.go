// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cloud

import "github.com/aws/aws-sdk-go-v2/aws"

// awsCfg wraps aws.Config so the check registrations stay readable.
type awsCfg struct{ aws.Config }
