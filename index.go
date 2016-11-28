package admin

type link struct {
	Name        string
	Path        string
	Description string
}

type linkSlice []link

func (p linkSlice) Len() int {
	return len(p)
}
func (p linkSlice) Less(i, j int) bool {
	return p[i].Path < p[j].Path
}
func (p linkSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type indexContext struct {
	Links   []link
	AppName string
}

const indexTemplate = `
<!DOCTYPE html>
<html>
<head>
	<title>{{ .AppName }}</title>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
	<link href="https://fonts.googleapis.com/css?family=Lato:400,700" rel="stylesheet">
	<style>
		body {
			font-family: 'Lato', sans-serif;
		}

		td {
			padding-top: 0.5em;
		}
	</style>
</head>
<body>
  <div class="container">
		<h1>admin page {{ if ne .AppName "" }} for {{ .AppName }}{{ end }}</h1>
		<table>
			{{ range $link := .Links }}
				<tr>
					<td style="padding-right:1.5em"><a href='{{ $link.Path }}'>{{ $link.Name }}</a></td>
					<td>{{ $link.Description }}</td>
				</tr>
			{{ end }}
		</table>
	</div>
</body>
</html>`
