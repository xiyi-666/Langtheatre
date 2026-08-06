package store

import "strings"

const miniProgramUseActivityPrefix = "MINI_AD_USE:"

func miniProgramUseActivity(activity string) string {
	return miniProgramUseActivityPrefix + strings.TrimSpace(activity)
}
