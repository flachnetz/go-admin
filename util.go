package admin

import (
	"reflect"
	"net/http"
	"encoding/json"
	"path"
)


// This method returns a handler, that produces json obtained from the given
// input value. The input value might either be just some ordinary value that
// can be marshaled using json.Marshal or a parameterless function that returns
// a value. In that case, the method is invoked and the result is marshaled
// to json.
func genericContentAsJSON(input interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value := evaluateIfFunc(input)
		writeJSON(w, http.StatusOK, value)
	}
}

// Evaluates the given value if it is a function. It is called and the first
// result value is returned by this function. If not, the input value is just
// returned without being modified.
func evaluateIfFunc(input interface{}) interface{} {
	value := reflect.ValueOf(input)
	switch value.Kind() {
	case reflect.Func:
		result := value.Call([]reflect.Value{})
		if len(result) < 1 {
			panic("Method did not return a value")
		}

		return result[0].Interface()
	default:
		return input
	}
}

func pathOf(firstComponent string, components ...string) string {
	joined := "/" + firstComponent
	for _, component := range components {
		joined += "/" + component
	}

	cleaned := path.Clean(joined)
	if cleaned == "." {
		cleaned = "/"
	}

	return cleaned
}

// marshals the given value to json and writes the result to the response.
func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

