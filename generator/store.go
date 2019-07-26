package generator

const storeTmpl = `
{{ define "store" }}

{{ template "store-Struct" . }}
{{ template "store-New" . }}
{{ template "store-NewWithTx" . }}
{{ template "store-Insert" . }}
{{ template "store-Get" . }}
{{ template "store-Delete" . }}
{{ template "store-List" . }}
{{ template "store-Replace" . }}
{{ end }}
`

const storeStructTmpl = `
{{ define "store-Struct" }}
{{- $fl := .FirstLetter -}}
{{- $structName := .Name -}}

// {{$structName}}Store manages the table. It provides several typed helpers
// that simplify common operations.
type {{$structName}}Store struct {
	*genji.Store
}
{{ end }}
`

const storeNewTmpl = `
{{ define "store-New" }}
{{- $fl := .FirstLetter -}}
{{- $structName := .Name -}}
{{- $tableName := .TableName -}}

// {{.NameWithPrefix "New"}}Store creates a {{$structName}}Store.
func {{.NameWithPrefix "New"}}Store(db *genji.DB) *{{$structName}}Store {
	var schema *record.Schema
	{{- if .Schema}}
	schema = &record.Schema{
		Fields: []field.Field{
		{{- range .Fields}}
			{Name: "{{.Name}}", Type: field.{{.Type}}},
		{{- end}}
		},
	}
	{{- end}}

	var indexes []string
	{{- if .HasIndexes }}
		{{- range .Indexes }}
		indexes = append(indexes, "{{.}}")
		{{- end }}
	{{- end }}

	return &{{$structName}}Store{Store: genji.NewStore(db, "{{$tableName}}", schema, indexes)}
}
{{ end }}
`

const storeNewWithTxTmpl = `
{{ define "store-NewWithTx" }}
{{- $fl := .FirstLetter -}}
{{- $structName := .Name -}}
{{- $tableName := .TableName -}}

// {{.NameWithPrefix "New"}}StoreWithTx creates a {{$structName}}Store valid for the lifetime of the given transaction.
func {{.NameWithPrefix "New"}}StoreWithTx(tx *genji.Tx) *{{$structName}}Store {
	var schema *record.Schema
	{{- if .Schema}}
	schema = &record.Schema{
		Fields: []field.Field{
		{{- range .Fields}}
			{Name: "{{.Name}}", Type: field.{{.Type}}},
		{{- end}}
		},
	}
	{{- end}}

	var indexes []string
	{{- if .HasIndexes }}
		{{ range .Indexes }}
		indexes = append(indexes, "{{.}}")
		{{- end }}
	{{- end }}

	return &{{$structName}}Store{Store: genji.NewStoreWithTx(tx, "{{$tableName}}", schema, indexes)}
}
{{ end }}
`

const storeInsertTmpl = `
{{ define "store-Insert" }}
{{- $fl := .FirstLetter -}}
{{- $structName := .Name -}}
// Insert a record in the table and return the primary key.
{{- if eq .Pk.Name ""}}
func ({{$fl}} *{{$structName}}Store) Insert(record *{{$structName}}) (recordID []byte, err error) {
	return {{$fl}}.Store.Insert(record)
}
{{- else }}
func ({{$fl}} *{{$structName}}Store) Insert(record *{{$structName}}) (err error) {
	_, err = {{$fl}}.Store.Insert(record)
	return err
}
{{- end}}
{{ end }}
`

const storeGetTmpl = `
{{ define "store-Get" }}
{{- $fl := .FirstLetter -}}
{{- $structName := .Name -}}
// Get a record using its primary key.
// If the record doesn't exist, returns table.ErrRecordNotFound.
{{- if eq .Pk.Name ""}}
func ({{$fl}} *{{$structName}}Store) Get(recordID []byte) (*{{$structName}}, error) {
{{- else}}
func ({{$fl}} *{{$structName}}Store) Get(pk {{.Pk.GoType}}) (*{{$structName}}, error) {
		recordID := field.Encode{{.Pk.Type}}(pk)
	{{- end}}
	rec, err := {{$fl}}.Store.Get(recordID)
	if err != nil {
		return nil, err
	}

	if v, ok := rec.(*{{$structName}}); ok {
		return v, nil
	}

	var record {{$structName}}

	err = record.ScanRecord(rec)
	if err != nil {
		return nil, err
	}

	return &record, nil
}
{{ end }}
`

const storeDeleteTmpl = `
{{ define "store-Delete" }}
{{- $fl := .FirstLetter -}}
{{- $structName := .Name -}}

// Delete a record using its primary key.
// If the record doesn't exist, returns table.ErrRecordNotFound.
{{- if ne .Pk.Name ""}}
func ({{$fl}} *{{$structName}}Store) Delete(pk {{.Pk.GoType}}) error {
	recordID := field.Encode{{.Pk.Type}}(pk)
	return {{$fl}}.Store.Delete(recordID)
}
{{- else }}
func ({{$fl}} *{{$structName}}Store) Delete(recordID []byte) error {
	return {{$fl}}.Store.Delete(recordID)
}
{{- end}}
{{ end }}
`

const storeListTmpl = `
{{ define "store-List" }}
{{- $fl := .FirstLetter -}}
{{- $structName := .Name -}}
// List records from the specified offset. If the limit is equal to -1, it returns all records after the selected offset.
func ({{$fl}} *{{$structName}}Store) List(offset, limit int) ([]{{$structName}}, error) {
	size := limit
	if size == -1 {
		size = 0
	}
	list := make([]{{$structName}}, 0, size)
	err := {{$fl}}.Store.List(offset, limit, func(recordID []byte, r record.Record) error {
		var record {{$structName}}
		err := record.ScanRecord(r)
		if err != nil {
			return err
		}
		list = append(list, record)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return list, nil
}
{{ end }}
`

const storeReplaceTmpl = `
{{ define "store-Replace" }}
{{- $fl := .FirstLetter -}}
{{- $structName := .Name -}}
// Replace the selected record by the given one.
{{- if eq .Pk.Name ""}}
func ({{$fl}} *{{$structName}}Store) Replace(recordID []byte, record *{{$structName}}) error {
{{- else}}
func ({{$fl}} *{{$structName}}Store) Replace(pk {{.Pk.GoType}}, record *{{$structName}}) error {
	recordID := field.Encode{{.Pk.Type}}(pk)
	if record.{{ .Pk.Name }} != pk {
		record.{{ .Pk.Name }} = pk
	}
{{- end}}
	return {{$fl}}.Store.Replace(recordID, record)
}
{{ end }}
`
