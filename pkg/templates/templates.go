package templates

import (
    "encoding/json"
    "html/template"
    "strings"
    "os"
    "path/filepath"
)

// RenderTemplateFromJSON reads JSON data, applies it to an html/template at templatePath,
// and writes the rendered output to outputPath. It returns an error if any step fails.
//
// jsonData can be a raw JSON string or []byte; typically you'll pass []byte.
func RenderTemplateFromJSON(jsonData []byte, templatePath, outputPath string) error {
    // 1) Decode JSON into a flexible container (map[string]any)
    var data map[string]any
    if err := json.Unmarshal(jsonData, &data); err != nil {
        return err
    }

    // 2) Parse the template
    // Use template.New with a name derived from the file for better error messages
    tmplName := filepath.Base(templatePath)
    tmpl, err := template.New(tmplName).Funcs(template.FuncMap{
        // Add any template helper functions here if needed
        "upper": func(s string) string { return strings.ToUpper(s) },
    }).ParseFiles(templatePath)
    if err != nil {
        return err
    }

    // 3) Create (or truncate) the output file
    outFile, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer func() {
        _ = outFile.Close()
    }()

    // 4) Execute the template with decoded data
    // If your template defines a named template (e.g., {{define "main"}}), call tmpl.ExecuteTemplate.
    // Otherwise tmpl.Execute is fine.
    if err := tmpl.Execute(outFile, data); err != nil {
        // If writing partially succeeded, ensure we don't leave a corrupt file lying around
        // (optional cleanup strategy)
        _ = outFile.Close()
        _ = os.Remove(outputPath)
        return err
    }

    // 5) (Optional) fsync to be extra safe on some filesystems
    _ = outFile.Sync()

    return nil
}
