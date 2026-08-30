// Package ext is the public API surface available to yaegi-loaded extensions.
//
// Extensions live under ~/.phi/extensions/ and <cwd>/.phi/extensions/ and export:
//
//	func Extension(phi *ext.API)
//
// See doc/extensions.md.
package ext
