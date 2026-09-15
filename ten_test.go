package ten

import (
	"bytes"
	"cmp"
	"strings"
	"testing"
)

func TestExecute(t *testing.T) {

	tt := []struct {
		desc string
		give string
		then string
	}{
		{
			give: `{{ 123 }}`,
			then: `123`,
		},
		{
			give: `{{ true }}{{ false }}`,
			then: `truefalse`,
		},
		{
			give: `{{ "abc\nd" }}`,
			then: "abc\nd",
		},
		{
			give: `{{ {} }}`,
			then: `{}`,
		},
		{
			give: `{{ [1, 2] }}`,
			then: `[1, 2]`,
		},
		{
			give: `{{ . }}`,
			then: `<nil>`,
		},
		{
			give: `{{ . = 3 }}{{ . }}`,
			then: `3`,
		},
		{
			give: `{{ a = 3 }}{{ a }}`,
			then: `3`,
		},
		{
			give: `{{ . = { "b": 3 } }}{{ .b }}`,
			then: `3`,
		},
		{
			give: `{{ . = [3] }}{{ .[0] }}`,
			then: `3`,
		},
		{
			give: `{{ . = [{ "b": 3 }] }}{{ .[0].b }}`,
			then: `3`,
		},
		{
			give: `{{ . = [{ "b": [{ "b": 3 }] }] }}{{ .[0].b[0].b }}`,
			then: `3`,
		},
		{
			give: `{{ a = { "b": 3 } }}{{ a.b }}`,
			then: `3`,
		},
		{
			give: `{{ a = { "b": [3] } }}{{ a.b[0] }}`,
			then: `3`,
		},
		{
			give: `{{ a = { "b": [{ "b": [3] }] } }}{{ a.b[0].b[0] }}`,
			then: `3`,
		},
		{
			give: `{{ a = { "b": 3 } }}{{ b = a.b }}{{ b }}`,
			then: `3`,
		},
		{
			give: "{{ for [2, 3] }}    Item: {{ . }}\n{{ end }}",
			then: "    Item: 2\n    Item: 3\n",
		},
		{
			give: "{{ for a : [2, 3] }}    Item: {{ a }}\n{{ end }}",
			then: "    Item: 2\n    Item: 3\n",
		},
		{
			give: "{{ for a : [2, 3] }}{{ for b : [4, 5] }}    A: {{ a }} B: {{ b }}\n{{ end }}{{ end }}",
			then: "    A: 2 B: 4\n    A: 2 B: 5\n    A: 3 B: 4\n    A: 3 B: 5\n",
		},
		{
			give: "{{ for a : [2, 3] }}{{ for b : [4, 5] }}{{ for [6, 7] }}    A: {{ a }} B: {{ b }} C: {{ . }}\n{{ end }}{{ end }}{{ end }}",
			then: "    A: 2 B: 4 C: 6\n    A: 2 B: 4 C: 7\n    A: 2 B: 5 C: 6\n    A: 2 B: 5 C: 7\n    A: 3 B: 4 C: 6\n    A: 3 B: 4 C: 7\n    A: 3 B: 5 C: 6\n    A: 3 B: 5 C: 7\n",
		},
		{
			give: "{{ for a, i : [2, 3] }}{{ for b, j : [4, 5] }}A:{{ a }} I:{{ i }} B:{{ b }} J:{{ j }}\n{{ end }}{{ end }}",
			then: "A:2 I:0 B:4 J:0\nA:2 I:0 B:5 J:1\nA:3 I:1 B:4 J:0\nA:3 I:1 B:5 J:1\n",
		},
		{
			give: `{{ . = [2, 3] }}{{ for . }}{{ . }}{{ end }}`,
			then: "23",
		},
		{
			give: `{{ . = { "a": [2, 3] } }}{{ for .a }}{{ . }}{{ end }}`,
			then: "23",
		},
		{
			give: "{{ if true }}yes{{ else }}no{{ end }}",
			then: "yes",
		},
		{
			give: "{{ if false }}yes{{ else }}no{{ end }}",
			then: "no",
		},
		{
			give: "{{ if true }}yes{{ end }}",
			then: "yes",
		},
		{
			give: "{{ if 1 }}yes{{ else }}no{{ end }}",
			then: "yes",
		},
		{
			give: "{{ if 0 }}yes{{ else }}no{{ end }}",
			then: "no",
		},
		{
			give: "{{ a = 1 }}{{ if a }}yes{{ else }}no{{ end }}",
			then: "yes",
		},
		{
			give: "{{ a = 0 }}{{ if a }}yes{{ else }}no{{ end }}",
			then: "no",
		},
		{
			give: "{{ if false }}yes{{ end }}",
			then: "",
		},
		{
			give: "{{ if true & true }}yes{{ else }}no{{ end }}",
			then: "yes",
		},
		{
			give: "{{ if true & true & false }}yes{{ else }}no{{ end }}",
			then: "no",
		},
		{
			give: "{{ if true & (false | true) }}yes{{ else }}no{{ end }}",
			then: "yes",
		},
		{
			give: "{{ for [true, false, false, true] }}{{ if . }}yes{{ else }}no{{ end }}{{ end }}",
			then: "yesnonoyes",
		},
		{
			give: "{{ false | true }}",
			then: "true",
		},
		{
			give: "{{ true & false }}",
			then: "false",
		},
		{
			give: "{{ true & true == false | true != false }}",
			then: "true",
		},
		{
			desc: "Test array index access",
			give: `{{ a = [1, 2] }}{{ a[0] }} {{ a[1] }}`,
			then: "1 2",
		},
		{
			desc: "Test array index access in a for",
			give: `{{ for v : [ ["A", "B"], [1, 2] ]}}(V:{{ v[0] }} D:{{ v[1] }}){{ end }}`,
			then: "(V:A D:B)(V:1 D:2)",
		},
		{
			desc: "Test trim space with -",
			give: strings.Join([]string{
				"{{ for a : [2, 3] -}}",
				"    {{ for b : [4, 5] -}}",
				`        {{ a }} {{ b }}{{ "\n" -}}`,
				"    {{ end -}}",
				"{{ end }}",
			}, "\n"),
			then: "2 4\n2 5\n3 4\n3 5\n",
		},
		{
			give: "{{ 1 + 2 }}",
			then: "3",
		},
		{
			give: "{{ 2 * 3 }}",
			then: "6",
		},
		{
			give: "{{ 2 * (3 + 1) }}",
			then: "8",
		},
		{
			give: "{{ 2 - 3 }}",
			then: "-1",
		},
		{
			give: "{{ 2 - -3 }}",
			then: "5",
		},
		{
			give: "{{ 2 - -(-3) }}",
			then: "-1",
		},
		{
			give: "{{ 2 * 3 }}",
			then: "6",
		},
		{
			give: "{{ 9 / 3 }}",
			then: "3",
		},
		{
			give: "{{ 3 == 3 }}",
			then: "true",
		},
		{
			give: "{{ 3 != 3 }}",
			then: "false",
		},
		{
			give: "{{ 1 > false }}",
			then: "false",
		},
		{
			give: "{{ 1 > true }}",
			then: "false",
		},
		{
			give: "{{ 1 == true }}",
			then: "false",
		},
		{
			give: "{{ 1 + 0 == true & true }}",
			then: "false",
		},
		{
			give: "{{ 1 + 0 != true & true }}",
			then: "false",
		},
		{
			give: "{{ 1 + 1 > true & true }}",
			then: "false",
		},
		{
			give: `{{ a = { "b": 1 } }}{{ 1 + a.b > 1 }}{{ 1 + a.b }}`,
			then: "true2",
		},
		{
			give: "a",
			then: "a",
		},
		{
			give: `{{ a = "Hi" }}{{ if a }}yes{{ end }}`,
			then: "yes",
		},
		{
			give: `{{ a = "" }}{{ if a }}yes{{ else }}no{{ end }}`,
			then: "no",
		},
		{
			give: `{{ for v, i : [1, 2] }}{{ if i == 0 }}a{{i}}{{end}}{{end}}`,
			then: `a0`,
		},
		{
			give: `{{ "a" == "a" }}{{ "a" == "b" }}`,
			then: `truefalse`,
		},
		{
			give: `{{ "a" != "b" }}{{ "a" == "b" }}`,
			then: `truefalse`,
		},
		{
			give: `{{ "b" > "a" }}`,
			then: `true`,
		},
	}

	for _, tc := range tt {

		p, err := Compile(tc.give)
		if err != nil {
			t.Fatal(err)
		}

		var got bytes.Buffer
		p.Execute(nil, &got)

		if tc.then != got.String() {
			p.print(p, 0)
			t.Errorf("\nMsg:\n%q\nGot:\n%q\nExp:\n%q\n", cmp.Or(tc.desc, tc.give), got.String(), tc.then)
		}
	}
}

func TestExecuteWithData(t *testing.T) {

	tt := []struct {
		desc string
		give string
		when any
		then string
	}{
		{
			give: `{{ . }}`,
			when: 3,
			then: `3`,
		},
		{
			give: `{{ . }}`,
			when: true,
			then: `true`,
		},
		{
			give: `{{ .A }}`,
			when: struct{ A int }{A: 3},
			then: `3`,
		},
		{
			give: `{{ .A }}`,
			when: &struct{ A int }{A: 3},
			then: `3`,
		},
		{
			give: `{{ .A.B }}`,
			when: &struct{ A struct{ B int } }{A: struct{ B int }{B: 3}},
			then: `3`,
		},
	}

	for _, tc := range tt {

		p, err := Compile(tc.give)
		if err != nil {
			t.Fatal(err)
		}

		var got bytes.Buffer
		p.Execute(tc.when, &got)

		if tc.then != got.String() {
			t.Errorf("\nMsg:\n%q\nGot:\n%q\nExp:\n%q\n", cmp.Or(tc.desc, tc.give), got.String(), tc.then)
		}
	}
}
