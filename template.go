package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"regexp"
	"gopkg.in/flosch/pongo2.v3"
)

// Interface with the original deepGet to use it with pongo2 values.
func deepGetValue(item *pongo2.Value, path string) *pongo2.Value {
	return pongo2.AsValue(deepGet(item.Interface(), path))
}

// File exists
func exists(path *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	_, err := os.Stat(path.String())

	if err == nil {
		return pongo2.AsValue(true), nil
	}

	if os.IsNotExist(err) {
		return pongo2.AsValue(false), nil
	}

	return pongo2.AsValue(false), &pongo2.Error{Sender: "filter:exists", ErrorMsg: "Error"}
}


// Groups by a deep key and separate their content further with the given separator.
// eg: value|groupByMulti:"my.deep.key|," (pipe is used to separate the separator from the key name)
func groupByMulti(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
	splitted := strings.SplitN(param.String(), "|", 2) // séparateur est le deuxième argument.

	groups := make(map[string][]interface{})

	fetch_key := splitted[0]
	separator := splitted[1]

	in.Iterate(func (idx, count int, value *pongo2.Value, unused *pongo2.Value) bool {
		val := deepGetValue(value, fetch_key)

		if (val.IsString() && val.String() != "") {
			// println(val.String())
			items := strings.Split(val.String(), separator)
			for _, item := range items {
				groups[item] = append(groups[item], value.Interface())
			}
		}

		return true
	}, func () {
		// empty.
	})

	return pongo2.AsValue(groups), nil
}


// groupBy groups a list of *RuntimeContainers by the path property key
func groupBy(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
	groups := make(map[string][]interface{})
	key := param.String()

	in.Iterate(func (idx, count int, value *pongo2.Value, unused *pongo2.Value) bool {
		val := deepGetValue(value, key)

		if (val.IsString() && val.String() != "") {
			// println(val.String())
			groups[val.String()] = append(groups[val.String()], value.Interface())
		}

		return true
	}, func () {
		// empty.
	})

	return pongo2.AsValue(groups), nil
}

// groupByKeys is the same as groupBy but only returns a list of keys
func mapKeys(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
	ret := []string{}

	in.Iterate(func (idx, count int, key *pongo2.Value, unused *pongo2.Value) bool {
		if (key.IsString() && key.String() != "") {
			ret = append(ret, key.String())
		}

		return true
	}, func () {
		// empty.
	})

	return pongo2.AsValue(ret), nil
}

// hasPrefix returns whether a given string is a prefix of another string
func hasPrefix(prefix, s string) bool {
	return strings.HasPrefix(s, prefix)
}

// hasSuffix returns whether a given string is a suffix of another string
func hasSuffix(suffix, s string) bool {
	return strings.HasSuffix(s, suffix)
}


func hashSha1(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	h := sha1.New()
	io.WriteString(h, in.String())
	return pongo2.AsValue(fmt.Sprintf("%x", h.Sum(nil))), nil
}

// arrayFirst returns first item in the array or nil if the
// input is nil or empty
func first(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if (!in.CanSlice()) {
		return nil, &pongo2.Error{Sender:"filter:first", ErrorMsg: "not slicable"}
	}

	var val *pongo2.Value

	in.Iterate(func (idx, count int, value *pongo2.Value, unused *pongo2.Value) bool {
		val = value
		return false
	}, func() {})

	return val, nil
}

// arrayLast returns last item in the array
func last(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if (!in.CanSlice()) {
		return nil, &pongo2.Error{Sender:"filter:last", ErrorMsg: "not slicable"}
	}

	var val *pongo2.Value

	in.Iterate(func (idx, count int, value *pongo2.Value, unused *pongo2.Value) bool {
		if (idx == count - 1) {
			val = value
			return false
		}
		return true
	}, func() {})

	return val, nil
}

// dirList returns a list of files in the specified path
func dirList(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {

	names := []string{}

	files, err := ioutil.ReadDir(in.String())

	if err != nil {
		return pongo2.AsValue(names), &pongo2.Error{Sender: "filter:dirList", ErrorMsg: err.Error()}
	}

	for _, f := range files {
		names = append(names, f.Name())
	}

	return pongo2.AsValue(names), nil
}


// trimPrefix returns whether a given string is a prefix of another string
func trimPrefix(in *pongo2.Value, prefix *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.TrimPrefix(in.String(), prefix.String())), nil
}


// trimSuffix returns whether a given string is a prefix of another string
func trimSuffix(in *pongo2.Value, prefix *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.TrimSuffix(in.String(), prefix.String())), nil
}


func jsonDecode(in *pongo2.Value, prefix *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	// Getting the raw buffer to play with.
	b := []byte(in.String())
	var result interface{}

	err := json.Unmarshal(b, &result)
	if (err != nil) {
		return pongo2.AsValue(nil), &pongo2.Error{Sender: "filter:jsonDecode", ErrorMsg: err.Error()}
	}

	return pongo2.AsValue(result), nil
}

/**
 * Special tag to avoid empty lines, which are trimmed to a single \n char.
 */

var re_emptylines = regexp.MustCompile(`([\s]*\r?\n){2,}`)
type tagMuteNode struct {
	wrapper *pongo2.NodeWrapper
}

func (self *tagMuteNode) Execute(ctx *pongo2.ExecutionContext, buf *bytes.Buffer) *pongo2.Error {
	b := bytes.NewBuffer(make([]byte, 0, 1024))
	err := self.wrapper.Execute(ctx, b)

	s2 := re_emptylines.ReplaceAllString(b.String(), "\n\n")
	buf.WriteString(s2)

	return err
}


func tagMuteParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagmuteNode := &tagMuteNode{}

	wrapper, _, err := doc.WrapUntilTag("endmute")
	if err != nil {
		return nil, err
	}

	tagmuteNode.wrapper = wrapper

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed mute-tag arguments.", nil)
	}

	return tagmuteNode, nil
}


////////////////////////////////////////////////////////////////////////////////

func init() {
	pongo2.RegisterFilter("groupByMulti", groupByMulti)
	pongo2.RegisterFilter("groupBy", groupBy)
	pongo2.RegisterFilter("keys", mapKeys)
	pongo2.RegisterFilter("exists", exists)
	pongo2.RegisterFilter("trimSuffix", trimSuffix)
	pongo2.RegisterFilter("trimPrefix", trimPrefix)
	pongo2.RegisterFilter("dirList", dirList)
	pongo2.RegisterFilter("jsonDecode", jsonDecode)

	pongo2.RegisterTag("mute", tagMuteParser)
}


func generateFile(config Config, containers Context) bool {
	templatePath := config.Template

	tmpl, err := pongo2.FromFile(templatePath)

	if err != nil {
		log.Fatalf("unable to parse template: %s", err)
	}

	filteredContainers := Context{}
	if config.OnlyPublished {
		for _, container := range containers {
			if len(container.PublishedAddresses()) > 0 {
				filteredContainers = append(filteredContainers, container)
			}
		}
	} else if config.OnlyExposed {
		for _, container := range containers {
			if len(container.Addresses) > 0 {
				filteredContainers = append(filteredContainers, container)
			}
		}
	} else {
		filteredContainers = containers
	}

	dest := os.Stdout
	if config.Dest != "" {
		dest, err = ioutil.TempFile(filepath.Dir(config.Dest), "docker-gen")
		defer func() {
			dest.Close()
			os.Remove(dest.Name())
		}()
		if err != nil {
			log.Fatalf("unable to create temp file: %s\n", err)
		}
	}

	var buf bytes.Buffer
	multiwriter := io.MultiWriter(dest, &buf)

	out := ""

	out, err = tmpl.Execute(pongo2.Context{"containers": filteredContainers})
	if err != nil {
		log.Fatalf("template error: %s\n", err)
	}

	io.WriteString(multiwriter, out)

	if config.Dest != "" {

		contents := []byte{}
		if fi, err := os.Stat(config.Dest); err == nil {
			if err := dest.Chmod(fi.Mode()); err != nil {
				log.Fatalf("unable to chmod temp file: %s\n", err)
			}
			if err := dest.Chown(int(fi.Sys().(*syscall.Stat_t).Uid), int(fi.Sys().(*syscall.Stat_t).Gid)); err != nil {
				log.Fatalf("unable to chown temp file: %s\n", err)
			}
			contents, err = ioutil.ReadFile(config.Dest)
			if err != nil {
				log.Fatalf("unable to compare current file contents: %s: %s\n", config.Dest, err)
			}
		}

		if bytes.Compare(contents, buf.Bytes()) != 0 {
			err = os.Rename(dest.Name(), config.Dest)
			if err != nil {
				log.Fatalf("unable to create dest file %s: %s\n", config.Dest, err)
			}
			log.Printf("Generated '%s' from %d containers", config.Dest, len(filteredContainers))
			return true
		}
		return false
	}
	return true
}
