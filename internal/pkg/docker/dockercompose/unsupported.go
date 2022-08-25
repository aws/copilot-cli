// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

// IgnoredKeys stores a list of keys in the Compose YAML that couldn't be processed,
// but are likely to not be significant enough to cause the converted application to
// fail. It's expected that this will eventually be displayed to the user.
type IgnoredKeys []string

// ignoredServiceKeys lists out the keys on Compose services that are ignored in conversion.
//
// note: build keys are handled separately in convertBuildConfig
var ignoredServiceKeys = map[string]bool{
	"blkio_config":        true,
	"cpu_count":           true,
	"cpu_percent":         true,
	"cpu_shares":          true,
	"cpu_period":          true,
	"cpu_quota":           true,
	"cpu_rt_runtime":      true,
	"cpu_rt_period":       true,
	"cpus":                true,
	"cpuset":              true,
	"cap_add":             true,
	"cap_drop":            true,
	"cgroup_parent":       true,
	"device_cgroup_rules": true,
	"logging":             true,
	"mac_address":         true,
	"mem_limit":           true,
	"mem_reservation":     true,
	"mem_swappiness":      true,
	"memswap_limit":       true,
	"oom_kill_disable":    true,
	"oom_score_adj":       true,
	"pid":                 true,
	"pids_limit":          true,
	"profiles":            true,
	"pull_policy":         true,
	"runtime":             true,
	"security_opt":        true,
	"shm_size":            true,
	"stdin_open":          true,
	"storage_opt":         true,
	"sysctls":             true,
	"tmpfs":               true,
	"user":                true,
	"userns_mode":         true,
	"hostname":            true,
	"depends_on":          true,
	"restart":             true,
	"read_only":           true,
	"ulimits":             true,
	// These aren't listed in the Compose spec, but are in the Compose structure...
	// are these legacy variants of existing properties?
	"custom_labels": true,
	"log_driver":    true,
	"log_opt":       true,
	"net":           true,
	"tty":           true,
	"uts":           true,
	"dockerfile":    true,
}

// fatalServiceKeys lists out the service keys that are unsupported and whose absence will
// break applications.
//
// note: build keys are handled separately in convertBuildConfig
// TODO(rclinard-amzn): Handle unsupported network keys when network conversion is implemented
var fatalServiceKeys = map[string]string{
	"credential_spec":   "",
	"devices":           "",
	"domainname":        "",
	"group_add":         "",
	"init":              "",
	"ipc":               "",
	"isolation":         "",
	"privileged":        "unsupported in Fargate",
	"external_links":    "",
	"working_dir":       "unsupported in Copilot manifests",
	"configs":           "unsupported, use secrets for similar functionality",
	"dns":               "unsupported in Copilot manifests",
	"dns_opt":           "unsupported in Copilot manifests",
	"dns_search":        "unsupported in Copilot manifests",
	"stop_grace_period": "unsupported in Copilot manifests",
	"stop_signal":       "unsupported in Copilot manifests",
	"volumes_from":      "sharing volumes is not yet supported",
	"volume_driver":     "Set the `driver` property on a volume instead",
	// Lifted in Milestone 4
	"volumes": "implemented in milestone 4",
	// Lifted in Milestone 5
	"secrets": "implemented in milestone 5",
	// Lifted in Milestone 6
	"networks": "implemented in Milestone 6",
	// Lifted in stretch goal
	"extra_hosts": "",
}
