// Package invoiceemail renders the restricted, user-editable invoice email format.
package invoiceemail

import (
	"bytes"
	"fmt"
	"html"
	htmltemplate "html/template"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/mayswind/ezbookkeeping/pkg/templates"
)

const MaxSubjectLength = 256
const MaxBodyLength = 32000
const DefaultSubjectTemplate = "Invoice {{invoice_number}} from {{business_name}}"
const DefaultBodyTemplate = `<h1><span data-invoice-variable="business_name">{{business_name}}</span> sent you an invoice</h1><p>Hello <span data-invoice-variable="contact_name">{{contact_name}}</span>,</p><p>Please find your invoice attached.</p><h2>Amount due: <span data-invoice-variable="amount_due">{{amount_due}}</span></h2><p>Due <span data-invoice-variable="due_date">{{due_date}}</span></p><div data-invoice-button="true" style="text-align: center">View Invoice</div><p>Thank you for your business.</p><p><strong><span data-invoice-variable="business_name">{{business_name}}</span></strong><br><span data-invoice-variable="business_address">{{business_address}}</span><br><span data-invoice-variable="business_phone">{{business_phone}}</span><br><span data-invoice-variable="business_website">{{business_website}}</span></p>`

// DefaultEstimateSubjectTemplate and DefaultEstimateBodyTemplate are the built-in preset worded for an estimate
const DefaultEstimateSubjectTemplate = "Estimate {{invoice_number}} from {{business_name}}"

var DefaultEstimateBodyTemplate = strings.NewReplacer(
	"sent you an invoice", "sent you an estimate",
	"Please find your invoice attached.", "Please find your estimate attached. You can accept or decline it online.",
	`Amount due: <span data-invoice-variable="amount_due">{{amount_due}}</span>`, `Estimate total: <span data-invoice-variable="invoice_total">{{invoice_total}}</span>`,
	`<p>Due <span data-invoice-variable="due_date">`, `<p>Valid until <span data-invoice-variable="due_date">`,
	">View Invoice</div>", ">View Estimate</div>",
).Replace(DefaultBodyTemplate)

// Variable is the stable key, English catalog label and resolved text for an editor.
type Variable struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// Result contains the same resolved subject, fragment and full document used by preview and mail.
type Result struct {
	Subject   string
	BodyHTML  string
	EmailHTML string
	Variables []Variable
}

var catalog = []Variable{
	{Key: "customer_name", Label: "Customer Name"}, {Key: "contact_name", Label: "Contact Name"}, {Key: "customer_email", Label: "Customer Email"},
	{Key: "invoice_number", Label: "Invoice Number"}, {Key: "invoice_date", Label: "Invoice Date"}, {Key: "due_date", Label: "Due Date"},
	{Key: "invoice_total", Label: "Invoice Total"}, {Key: "amount_paid", Label: "Amount Paid"}, {Key: "amount_due", Label: "Amount Due"},
	{Key: "currency", Label: "Currency"}, {Key: "invoice_url", Label: "Invoice URL"}, {Key: "business_name", Label: "Business Name"},
	{Key: "business_address", Label: "Business Address"}, {Key: "business_phone", Label: "Business Phone"}, {Key: "business_website", Label: "Business Website"},
	{Key: "document_type", Label: "Document Type"},
}

var emptyInline = regexp.MustCompile(`<(strong|b|em|i|u|span)>\s*</(strong|b|em|i|u|span)>`)
var repeatedBreaks = regexp.MustCompile(`(<br/>\s*){2,}`)
var edgeBreaks = regexp.MustCompile(`(<p[^>]*>)\s*<br/>|<br/>\s*(</p>)`)
var emptyParagraph = regexp.MustCompile(`<p[^>]*>\s*</p>`)

// tidyEmptyValues removes the blank lines left by empty variables, e.g. a footer without phone or website.
// ponytail: also collapses intentional double line breaks; track empty spans in writeNode if that matters.
func tidyEmptyValues(body string) string {
	body = emptyInline.ReplaceAllString(body, "")
	body = repeatedBreaks.ReplaceAllString(body, "<br/>")
	for previous := ""; previous != body; {
		previous = body
		body = edgeBreaks.ReplaceAllString(body, "$1$2")
	}
	return emptyParagraph.ReplaceAllString(body, "")
}

var tokens = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

func knownKey(key string) bool {
	for _, variable := range catalog {
		if variable.Key == key {
			return true
		}
	}
	return false
}

// EffectiveTemplates supplies the built-in preset for profiles that have never saved a template.
// A configured subject distinguishes an intentionally empty body from an unset template.
func EffectiveTemplates(subject, body string) (string, string) {
	if subject == "" {
		return DefaultSubjectTemplate, DefaultBodyTemplate
	}
	return subject, body
}

// PresetWithoutBusinessName drops the business phrasing from the built-in preset, as the old fixed email did.
func PresetWithoutBusinessName() (string, string) {
	return strings.Replace(DefaultSubjectTemplate, " from {{business_name}}", "", 1),
		strings.Replace(DefaultBodyTemplate, `<span data-invoice-variable="business_name">{{business_name}}</span> sent you an invoice`, "You have received an invoice", 1)
}

// EstimateTemplates rewords the built-in invoice preset (with or without the business name) for an estimate. Bodies are compared in
// their sanitized form, so the preset still matches after a round trip through the editor; a template the user wrote is returned unchanged.
func EstimateTemplates(subject, body string) (string, string) {
	bareSubject, bareBody := PresetWithoutBusinessName()

	switch strings.TrimSpace(subject) {
	case DefaultSubjectTemplate:
		subject = DefaultEstimateSubjectTemplate
	case bareSubject:
		subject = strings.Replace(DefaultEstimateSubjectTemplate, " from {{business_name}}", "", 1)
	}

	switch canonicalTemplate(body) {
	case canonicalTemplate(DefaultBodyTemplate):
		body = DefaultEstimateBodyTemplate
	case canonicalTemplate(bareBody):
		body = strings.Replace(DefaultEstimateBodyTemplate, `<span data-invoice-variable="business_name">{{business_name}}</span> sent you an estimate`, "You have received an estimate", 1)
	}

	return subject, body
}

// canonicalTemplate is a body as SanitizeTemplate stores it; an unparsable body is compared as it is
func canonicalTemplate(body string) string {
	sanitized, err := SanitizeTemplate(body)

	if err != nil {
		return body
	}

	return sanitized
}

// ValidateSubject checks the template and expanded subject without interpreting inserted values again.
func ValidateSubject(subject string) error {
	if err := validateSubjectLine(subject); err != nil {
		return err
	}
	_, err := expand(subject, nil)
	return err
}

func validateSubjectLine(subject string) error {
	if strings.TrimSpace(subject) == "" || strings.ContainsAny(subject, "\r\n") || utf8.RuneCountInString(subject) > MaxSubjectLength {
		return fmt.Errorf("email subject must be nonblank, single line and at most %d characters", MaxSubjectLength)
	}
	return nil
}

func expand(text string, values map[string]string) (string, error) {
	var firstErr error
	expanded := tokens.ReplaceAllStringFunc(text, func(token string) string {
		key := strings.TrimSpace(tokens.FindStringSubmatch(token)[1])
		if !knownKey(key) {
			if firstErr == nil {
				firstErr = fmt.Errorf("unknown invoice email variable: %s", key)
			}
			return ""
		}
		if values == nil {
			return "{{" + key + "}}"
		}
		return values[key]
	})
	return expanded, firstErr
}

// SanitizeTemplate strips unsupported HTML and serializes canonical variable/button markers.
func SanitizeTemplate(body string) (string, error) {
	sanitized, err := fragment(body, nil)
	if err != nil {
		return "", err
	}
	if utf8.RuneCountInString(sanitized) > MaxBodyLength {
		return "", fmt.Errorf("serialized email body must be at most %d characters", MaxBodyLength)
	}
	return sanitized, nil
}

// Render sanitizes then resolves the fragment, producing an email-compatible responsive document.
func Render(subject, body string, values map[string]string) (*Result, error) {
	if err := ValidateSubject(subject); err != nil {
		return nil, err
	}
	resolvedSubject, err := expand(subject, values)
	if err != nil {
		return nil, err
	}
	// Expanded user data can contain braces, but is never interpreted as another template.
	if strings.TrimSpace(resolvedSubject) == "" || strings.ContainsAny(resolvedSubject, "\r\n") || utf8.RuneCountInString(resolvedSubject) > MaxSubjectLength {
		return nil, fmt.Errorf("expanded email subject must be nonblank, single line and at most %d characters", MaxSubjectLength)
	}
	resolvedBody, err := fragment(body, values)
	if err != nil {
		return nil, err
	}
	return renderEmailDocument(resolvedSubject, tidyEmptyValues(resolvedBody), values)
}

func renderEmailDocument(subject, body string, values map[string]string) (*Result, error) {
	variables := make([]Variable, len(catalog))
	copy(variables, catalog)
	for index := range variables {
		variables[index].Value = values[variables[index].Key]
	}
	var document bytes.Buffer
	// Body has been parsed, allowlisted and context-escaped; the wrapper escapes the subject itself.
	documentTemplate, err := templates.GetTemplate(templates.TEMPLATE_INVOICE_EMAIL)
	if err != nil {
		return nil, err
	}
	if err := documentTemplate.Execute(&document, map[string]any{"Subject": subject, "Body": htmltemplate.HTML(body)}); err != nil {
		return nil, err
	}
	return &Result{Subject: subject, BodyHTML: body, EmailHTML: document.String(), Variables: variables}, nil
}

func fragment(body string, values map[string]string) (string, error) {
	if utf8.RuneCountInString(body) > MaxBodyLength {
		return "", fmt.Errorf("email body must be at most %d characters", MaxBodyLength)
	}
	nodes, err := xhtml.ParseFragment(strings.NewReader(body), &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return "", err
	}
	var result strings.Builder
	for _, node := range nodes {
		if err := writeNode(&result, node, values); err != nil {
			return "", err
		}
	}
	return result.String(), nil
}

func hasAttr(node *xhtml.Node, key string) bool {
	for _, attribute := range node.Attr {
		if attribute.Namespace == "" && attribute.Key == key {
			return true
		}
	}
	return false
}

func attr(node *xhtml.Node, key string) string {
	for _, a := range node.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val
		}
	}
	return ""
}
func alignment(node *xhtml.Node) string {
	for _, declaration := range strings.Split(attr(node, "style"), ";") {
		pair := strings.SplitN(declaration, ":", 2)
		if len(pair) == 2 && strings.TrimSpace(strings.ToLower(pair[0])) == "text-align" {
			switch value := strings.TrimSpace(strings.ToLower(pair[1])); value {
			case "left", "center", "right", "justify":
				return value
			}
		}
	}
	return ""
}
func plainText(node *xhtml.Node) string {
	if node.Type == xhtml.TextNode {
		return node.Data
	}
	if node.Type == xhtml.ElementNode {
		switch node.Data {
		case "script", "style", "iframe", "object", "embed", "svg", "math", "template":
			return ""
		}
	}
	var result strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		result.WriteString(plainText(child))
	}
	return result.String()
}
func escapedText(text string) string {
	if strings.TrimSpace(text) == "" && strings.Contains(text, "\n") {
		return " " // source formatting between tags, not a line break
	}
	return strings.ReplaceAll(html.EscapeString(strings.ReplaceAll(text, "\r\n", "\n")), "\n", "<br/>")
}
func safeURL(value string) bool {
	if strings.ContainsAny(value, "\r\n\t") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	case "mailto":
		return parsed.Opaque != "" || parsed.Path != ""
	}
	return false
}
func writeNode(out *strings.Builder, node *xhtml.Node, values map[string]string) error {
	if node.Type == xhtml.TextNode {
		text, err := expand(node.Data, values)
		if err != nil {
			return err
		}
		out.WriteString(escapedText(text))
		return nil
	}
	if node.Type != xhtml.ElementNode {
		return nil
	}
	switch node.Data {
	case "script", "style", "iframe", "object", "embed", "svg", "math", "template", "head", "title", "noscript":
		return nil
	}
	if node.Data == "span" && hasAttr(node, "data-invoice-variable") {
		key := attr(node, "data-invoice-variable")
		if !knownKey(key) {
			return fmt.Errorf("unknown invoice email variable: %s", key)
		}
		if values == nil {
			out.WriteString(`<span data-invoice-variable="` + key + `">{{` + key + `}}</span>`)
		} else {
			out.WriteString(escapedText(values[key]))
		}
		return nil
	}
	if node.Data == "div" && attr(node, "data-invoice-button") == "true" {
		label, err := expand(plainText(node), values)
		if err != nil {
			return err
		}
		if strings.TrimSpace(label) == "" {
			return fmt.Errorf("invoice button text must not be empty")
		}
		align := alignment(node)
		if align == "" {
			align = "center"
		}
		if values == nil {
			out.WriteString(`<div data-invoice-button="true" style="text-align: ` + align + `">` + html.EscapeString(label) + `</div>`)
			return nil
		}
		destination := values["invoice_url"]
		if destination != "" && !safeURL(destination) {
			return fmt.Errorf("invoice URL is invalid")
		}
		out.WriteString(`<div style="margin: 24px 0; text-align: ` + align + `"><a href="` + html.EscapeString(destination) + `" style="display: inline-block; padding: 12px 24px; border-radius: 4px; background-color: #3d86f5; color: #ffffff; text-decoration: none; font-weight: bold">` + html.EscapeString(label) + `</a></div>`)
		return nil
	}
	tag := node.Data
	switch tag {
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "strong", "b", "em", "i", "u", "ul", "ol", "li", "a", "br", "span":
	default:
		tag = ""
	}
	if tag != "" {
		out.WriteString("<" + tag)
		styles := []string{}
		if values != nil {
			switch tag {
			case "p":
				styles = append(styles, "margin: 0 0 16px")
			case "h1":
				styles = append(styles, "font-size: 28px; line-height: 1.25; margin: 0 0 20px")
			case "h2":
				styles = append(styles, "font-size: 22px; line-height: 1.3; margin: 0 0 16px")
			case "a":
				styles = append(styles, "color: #2563eb")
			}
		}
		if align := alignment(node); align != "" {
			styles = append(styles, "text-align: "+align)
		}
		if len(styles) > 0 {
			out.WriteString(` style="` + strings.Join(styles, "; ") + `"`)
		}
		if tag == "a" {
			href := attr(node, "href")
			expanded, err := expand(href, nil)
			if err != nil {
				return err
			}
			if expanded == "{{invoice_url}}" {
				if values != nil {
					expanded = values["invoice_url"]
				}
				if values == nil || safeURL(expanded) {
					out.WriteString(` href="` + html.EscapeString(expanded) + `"`)
				}
			} else if !strings.Contains(expanded, "{{") && safeURL(expanded) {
				out.WriteString(` href="` + html.EscapeString(expanded) + `"`)
			}
		}
		if tag == "br" {
			out.WriteString("/>")
			return nil
		}
		out.WriteString(">")
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if err := writeNode(out, child, values); err != nil {
			return err
		}
	}
	if tag != "" {
		out.WriteString("</" + tag + ">")
	}
	return nil
}

// RenderPlainText preserves legacy plain text subjects and message paragraphs.
func RenderPlainText(subject, message string, values map[string]string) (*Result, error) {
	if err := validateSubjectLine(subject); err != nil {
		return nil, err
	}
	var body strings.Builder
	for _, line := range strings.Split(message, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			body.WriteString(`<p style="margin: 0 0 16px">` + html.EscapeString(line) + `</p>`)
		}
	}
	summary, err := fragment(`<div data-invoice-button="true">View invoice</div><p>Invoice {{invoice_number}} · Amount due {{amount_due}} · Due {{due_date}}</p><p>{{invoice_url}}</p>`, values)
	if err != nil {
		return nil, err
	}
	body.WriteString(summary)
	return renderEmailDocument(subject, body.String(), values)
}
