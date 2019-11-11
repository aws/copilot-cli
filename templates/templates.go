//go:generate packr2

package templates

import "github.com/gobuffalo/packr/v2"

// Box can be used to read in templates from the templates directory.
// For example, templates.Box().FindString("environment/cf.yml").
// ==== Note about Custom Resources ====
// Custom resources from the cf-custom-resources directory are built,
// minified and coppied into custom-resources dir. You can refer to the files
// by their name. This is done as part of the build step. The minified files
// are removed after the binaries are built, since they'll be packed.
func Box() *packr.Box {
	return packr.New("templates", "./")
}
