package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	flag "github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v2"
)

const header = `# Values will be overwritten by helm hashtag.
# To follow a new tag, add an entry matching provided values such as:
#
#    app-chart:
#        yourImage: null
#
# and it will automatically be populated with the hash that app-chart.yourImage.tag
# points to.

`

func main() {
	var (
		resolver string
		tf       string
		vf       valueFiles
		vs       []string
	)
	flag.VarP(&vf, "values", "f", "specify values in a YAML file or a URL(can specify multiple)")
	flag.StringArrayVar(
		&vs, "set", []string{},
		"set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)",
	)
	flag.StringVar(&tf, "tagfile", "hashtags.yaml", "the hash tag value file to use for repository overrides")
	flag.StringVar(&resolver, "resolver", "", "the source of truth for resolving docker tags into digest hashes")
	flag.Parse()

	if resolver == "" {
		fmt.Println(
			"A resolver URL must be set. This should serve http GET <alias-url>/<tag>",
			"for resolving the docker tag digest. See the README for further information.")
		os.Exit(1)
	}

	tags := hashtags{}
	b, err := ioutil.ReadFile(tf)
	if err != nil {
		switch err.(type) {
		case *os.PathError:
		default:
			fmt.Printf("%T", err)
			fmt.Println(err)
			os.Exit(1)
		}
	}
	if err := yaml.Unmarshal(b, &tags); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	values, err := vals(vf, vs)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := tags.updateFrom(resolver, values); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	b, err = yaml.Marshal(tags)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	b = append([]byte(header), b...)
	if err = ioutil.WriteFile(tf, b, 0622); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type hashtags map[string]interface{}

func (t *hashtags) updateFrom(resolver string, values map[string]interface{}) error {
	// Find the intersection of which charts we know about and which are available.
	updates := 0
	total := 0
	for chart, refs := range *t {
		original, ok := values[chart].(map[interface{}]interface{})
		if !ok {
			fmt.Printf("WARNING: settings for char %s was not found in provided values\n", chart)
			continue
		}
		for imageRef := range refs.(map[interface{}]interface{}) {
			total++
			image, ok := original[imageRef].(map[interface{}]interface{})
			if !ok {
				fmt.Printf("WARNING: an image value was not found for %s.%s\n", chart, imageRef)
				continue
			}
			repo, ok := image["repository"].(string)
			if !ok {
				fmt.Printf("WARNING: an image repository value was not found for chart %s\n", chart)
				continue
			}
			parts := strings.Split(repo, "/")
			imageName := parts[len(parts)-1]

			var tag string
			switch v := image["tag"].(type) {
			case string:
				tag = v
			case int:
				tag = fmt.Sprintf("%d", v)
			default:
				fmt.Printf("WARNING: an image tag value was not found for chart %s\n", chart)
				continue
			}

			fmt.Printf("updating hash digest for tag %s of %s image %s\n", tag, chart, imageName)
			updates++
			if err := t.update(chart, imageRef.(string), repo, resolver, imageName, tag); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}
	fmt.Printf("%d of %d image tags updated\n", updates, total)
	return nil
}

func (t *hashtags) update(chart, imageRef, origRepo, resolver, imageName, tag string) error {
	url := fmt.Sprintf("%s/%s/%s", resolver, imageName, tag)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected resolver status code %d for %q", resp.StatusCode, url)
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "@")
		if len(parts) != 2 {
			return fmt.Errorf("unknown resolver format: %q", line)
		}
		// On no error, we hopefully got the hash digest in format
		// <repo>@<digest type>:<long hash>
		repo, digest := parts[0], parts[1]

		// Use this digest if this entry is about our original repository.
		if strings.HasPrefix(repo, origRepo) {
			if parts = strings.Split(digest, ":"); len(parts) != 2 {
				return fmt.Errorf("unknown digest format: %q", digest)
			}
			hashType, hash := parts[0], parts[1]
			m := map[string]interface{}(*t)
			m[chart].(map[interface{}]interface{})[imageRef] = map[string]string{
				"repository": fmt.Sprintf("%s@%s", origRepo, hashType),
				"tag":        hash,
			}
			*t = m
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return fmt.Errorf("could not resolve %s", origRepo)
}
