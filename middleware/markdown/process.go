package markdown

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/russross/blackfriday"
)

const (
	DefaultTemplate  = "defaultTemplate"
	DefaultStaticDir = "generated_site"
)

// Process processes the contents of a page in b. It parses the metadata
// (if any) and uses the template (if found).
func (md Markdown) Process(c Config, requestPath string, b []byte) ([]byte, error) {
	var metadata = Metadata{Variables: make(map[string]interface{})}
	var markdown []byte
	var err error

	// find parser compatible with page contents
	parser := findParser(b)

	if parser == nil {
		// if not found, assume whole file is markdown (no front matter)
		markdown = b
	} else {
		// if found, assume metadata present and parse.
		markdown, err = parser.Parse(b)
		if err != nil {
			return nil, err
		}
		metadata = parser.Metadata()
	}

	// if template is not specified, check if Default template is set
	if metadata.Template == "" {
		if _, ok := c.Templates[DefaultTemplate]; ok {
			metadata.Template = DefaultTemplate
		}
	}

	// if template is set, load it
	var tmpl []byte
	if metadata.Template != "" {
		if t, ok := c.Templates[metadata.Template]; ok {
			tmpl, err = ioutil.ReadFile(t)
		}
		if err != nil {
			return nil, err
		}
	}

	// process markdown
	markdown = blackfriday.Markdown(markdown, c.Renderer, 0)

	// set it as body for template
	metadata.Variables["markdown"] = string(markdown)

	return md.processTemplate(c, requestPath, tmpl, metadata)
}

// processTemplate processes a template given a requestPath,
// template (tmpl) and metadata
func (md Markdown) processTemplate(c Config, requestPath string, tmpl []byte, metadata Metadata) ([]byte, error) {
	// if template is not specified,
	// use the default template
	if tmpl == nil {
		tmpl = defaultTemplate(c, metadata, requestPath)
	}

	// process the template
	b := new(bytes.Buffer)
	t, err := template.New("").Parse(string(tmpl))
	if err != nil {
		return nil, err
	}
	if err = t.Execute(b, metadata.Variables); err != nil {
		return nil, err
	}

	// generate static page
	if err = md.generatePage(c, requestPath, b.Bytes()); err != nil {
		// if static page generation fails,
		// nothing fatal, only log the error.
		// TODO: Report this non-fatal error, but don't log it here
		log.Println(err)
	}

	return b.Bytes(), nil

}

// generatePage generates a static html page from the markdown in content if c.StaticDir
// is a non-empty value, meaning that the user enabled static site generation.
func (md Markdown) generatePage(c Config, requestPath string, content []byte) error {
	// Only generate the page if static site generation is enabled
	if c.StaticDir != "" {
		// if static directory is not existing, create it
		if _, err := os.Stat(c.StaticDir); err != nil {
			err := os.MkdirAll(c.StaticDir, os.FileMode(0755))
			if err != nil {
				return err
			}
		}

		filePath := filepath.Join(c.StaticDir, requestPath)

		// If it is index file, use the directory instead
		if md.IsIndexFile(filepath.Base(requestPath)) {
			filePath, _ = filepath.Split(filePath)
		}

		// Create the directory in case it is not existing
		if err := os.MkdirAll(filePath, os.FileMode(0744)); err != nil {
			return err
		}

		// generate index.html file in the directory
		filePath = filepath.Join(filePath, "index.html")
		err := ioutil.WriteFile(filePath, content, os.FileMode(0664))
		if err != nil {
			return err
		}

		c.StaticFiles[requestPath] = filePath
	}

	return nil
}

// defaultTemplate constructs a default template.
func defaultTemplate(c Config, metadata Metadata, requestPath string) []byte {
	var scripts, styles bytes.Buffer
	for _, style := range c.Styles {
		styles.WriteString(strings.Replace(cssTemplate, "{{url}}", style, 1))
		styles.WriteString("\r\n")
	}
	for _, script := range c.Scripts {
		scripts.WriteString(strings.Replace(jsTemplate, "{{url}}", script, 1))
		scripts.WriteString("\r\n")
	}

	// Title is first line (length-limited), otherwise filename
	title := metadata.Title
	if title == "" {
		title = filepath.Base(requestPath)
		if body, _ := metadata.Variables["markdown"].([]byte); len(body) > 128 {
			title = string(body[:128])
		} else if len(body) > 0 {
			title = string(body)
		}
	}

	html := []byte(htmlTemplate)
	html = bytes.Replace(html, []byte("{{title}}"), []byte(title), 1)
	html = bytes.Replace(html, []byte("{{css}}"), styles.Bytes(), 1)
	html = bytes.Replace(html, []byte("{{js}}"), scripts.Bytes(), 1)

	return html
}

const (
	htmlTemplate = `<!DOCTYPE html>
<html>
	<head>
		<title>{{title}}</title>
		<meta charset="utf-8">
		{{css}}
		{{js}}
	</head>
	<body>
		{{.markdown}}
	</body>
</html>`
	cssTemplate = `<link rel="stylesheet" href="{{url}}">`
	jsTemplate  = `<script src="{{url}}"></script>`
)
