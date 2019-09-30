//go:generate packr2

package templates

import "github.com/gobuffalo/packr/v2"

// Box can be used to read in templates from the templates directory.
// For example, templates.Box().FindString("environment/cf.yml")
func Box() *packr.Box {
	return packr.New("templates", "./")
}
