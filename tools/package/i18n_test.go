package main

import (
	"bytes"
	"testing"
)

// TestBundleLanguages 验证应用包声明语言目录中的源语言与支持语言
func TestBundleLanguages(t *testing.T) {
	plist := infoPlist(&config{Name: "GoPaste"})
	for _, entry := range [][]byte{
		[]byte("<key>CFBundleDevelopmentRegion</key>\n\t<string>zh-Hans</string>"),
		[]byte("<key>CFBundleLocalizations</key>\n\t<array>\n\t\t<string>zh-Hans</string>\n\t\t<string>en</string>\n\t</array>"),
	} {
		if !bytes.Contains(plist, entry) {
			t.Fatalf("应用包缺少语言声明：%s", entry)
		}
	}
}
