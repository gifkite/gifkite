package main

import (
	_ "embed"
)

//go:embed assets/gifkiteTemplate@2x.png
var trayTemplateBytes []byte

//go:embed assets/gifkiteTemplate.png
var trayTemplate1xBytes []byte

//go:embed assets/AppIcon-32.png
var appIconColorBytes []byte

func trayIconTemplate() []byte {
	return trayTemplateBytes
}

func trayIconColor() []byte {
	return appIconColorBytes
}

