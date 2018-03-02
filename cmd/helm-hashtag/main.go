package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
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
		gcpRepo string
		tf      string
		vf      valueFiles
		vs      []string
	)
	flag.VarP(&vf, "values", "f", "specify values in a YAML file or a URL(can specify multiple)")
	flag.StringArrayVar(
		&vs, "set", []string{},
		"set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)",
	)
	flag.StringVar(&tf, "tagfile", "hashtags.yaml", "the hash tag value file to use for repository overrides")
	flag.StringVar(&gcpRepo, "gcp-repo", "", "the source of truth for looking up docker tags, must be configured with gcloud cli")
	flag.Parse()

	if gcpRepo == "" {
		fmt.Println(
			"A GCP repository must be set. This should be accessible via the gcloud command as it will be used",
			"for looking up the docker tag digest.")
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
	if err := tags.updateFrom(gcpRepo, values); err != nil {
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

func (t *hashtags) updateFrom(gcpRepo string, values map[string]interface{}) error {
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
			if err := t.update(chart, imageRef.(string), repo, gcpRepo, imageName, tag); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}
	fmt.Printf("%d of %d image tags updated\n", updates, total)
	return nil
}

func (t *hashtags) update(chart, imageRef, origRepo, gcpRepo, imageName, tag string) error {
	cmd := exec.Command(
		"gcloud", "container", "images", "list-tags",
		fmt.Sprintf("%s/%s", gcpRepo, imageName),
		"--limit", "1", "--filter", fmt.Sprintf("tags:%s", tag),
		"--format", "get(digest)",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		return err
	}
	// On no error, we hopefully got the hash digest in format
	// <digest type>:<long hash>
	parts := strings.Split(string(out), ":")
	if len(parts) != 2 {
		return fmt.Errorf("unknown digest format: %q", string(out))
	}
	digest, hash := parts[0], parts[1]

	m := map[string]interface{}(*t)
	m[chart].(map[interface{}]interface{})[imageRef] = map[string]string{
		"repository": fmt.Sprintf("%s@%s", origRepo, digest),
		"tag":        hash,
	}
	*t = m
	return nil
}
