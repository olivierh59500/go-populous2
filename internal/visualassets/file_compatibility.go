package visualassets

// SaveAnimationToken maps a historical save-file value to imported artwork.
// The normal game does not consume these tokens or dispatch simulation with
// them. Several art roles can share one value and remain distinct aliases.
type SaveAnimationToken struct {
	Token     uint16 `json:"token"`
	Animation string `json:"animation"`
	Frame     int    `json:"frame"`
}

type FileCompatibility struct {
	AnimationTokens []SaveAnimationToken `json:"animation_tokens"`
}
