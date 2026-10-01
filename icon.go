package main

import (
	_ "embed"
)

//go:embed assets/gifkiteTemplate@2x.png
var trayTemplateBytes []byte

//go:embed assets/gifkiteTemplate.png
var trayTemplate1xBytes []byte

func trayIconTemplate() []byte {
	return trayTemplateBytes
}

func trayIconColor() []byte {
	return trayTemplateBytes
}

