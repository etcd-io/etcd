// Copyright 2025 The etcd Authors
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

package command

import "github.com/spf13/cobra"

const (
	groupKVID                 = "kv"
	groupClusterMaintenanceID = "cluster maintenance"
	groupConcurrencyID        = "concurrency"
	groupAuthenticationID     = "authentication"
	groupUtilityID            = "utility"
)

func NewKVGroup() *cobra.Group {
	return &cobra.Group{
		ID:    groupKVID,
		Title: "Key-value commands",
	}
}

func NewClusterMaintenanceGroup() *cobra.Group {
	return &cobra.Group{
		ID:    groupClusterMaintenanceID,
		Title: "Cluster maintenance commands",
	}
}

func NewConcurrencyGroup() *cobra.Group {
	return &cobra.Group{
		ID:    groupConcurrencyID,
		Title: "Concurrency commands",
	}
}

func NewAuthenticationGroup() *cobra.Group {
	return &cobra.Group{
		ID:    groupAuthenticationID,
		Title: "Authentication commands",
	}
}

func NewUtilityGroup() *cobra.Group {
	return &cobra.Group{
		ID:    groupUtilityID,
		Title: "Utility commands",
	}
}
