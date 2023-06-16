package upload

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

// FileByChunks writes contents of file into dest in page sized chunks
func FileByChunks(file io.Reader, dest io.Writer) error {
	// page sized buffer
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
		}

		// Write the file
		n, err = dest.Write(buf[:n])
		if err != nil {
			return fmt.Errorf("writing to file: %w", err)
		}

		// Done processing
		if n < len(buf) {
			return nil
		}
	}
	return nil
}

// PerformJsonMutation validates if contents of in contain valid json structure
// and then performs data mutation which is later written to out.
func PerformJsonMutation(in io.Reader, out io.Writer) error {

	contents, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("reading contents of file: %w", err)
	}

	inData := map[string]any{}
	outData := map[string]any{}

	if err := json.Unmarshal(contents, &inData); err != nil {
		return fmt.Errorf("unmarshalling file contents: %w", err)
	}

	// Perform processing of validated json
	for key, val := range inData {
		// Remove vowel properties
		if !startsWithVowel(key) {
			outData[key] = val

			// Ints which are even should be increased by a 1000
			if intVal, ok := val.(float64); ok {
				whole, frac := math.Modf(intVal)
				if int64(whole)%2 == 0 && !(frac > 0) {
					outData[key] = intVal + 1000
				}
			}
		}
	}

	outBytes, err := json.Marshal(outData)
	if err != nil {
		return fmt.Errorf("could not marshal outData: %w", err)
	}

	_, err = out.Write(outBytes)
	if err != nil {
		return fmt.Errorf("writing modified file: %w", err)
	}

	return nil
}

func startsWithVowel(str string) bool {
	vowels := []byte{'a', 'o', 'e', 'i', 'u'}
	firstLetter := strings.ToLower(str)[0]
	for _, v := range vowels {
		if firstLetter == v {
			return true
		}
	}
	return false
}
