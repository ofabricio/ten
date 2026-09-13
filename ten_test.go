package ten

import (
	"bytes"
	"cmp"
	"testing"
)

func TestExecute(t *testing.T) {

	tt := []struct {
		desc string
		give string
		then string
	}{
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
			give: "{{ if true }}yes{{ else }}no{{ end }}",
			then: "yes",
		},
		{
			give: "{{ if false }}yes{{ else }}no{{ end }}",
			then: "no",
		},
		{
			give: "{{ for [true, false, false, true] }}{{ if . }}yes{{ else }}no{{ end }}{{ end }}",
			then: "yesnonoyes",
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
	}

	for _, tc := range tt {

		p, err := Compile(tc.give)
		if err != nil {
			t.Fatal(err)
		}

		var got bytes.Buffer
		p.Execute(nil, &got)

		if tc.then != got.String() {
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
