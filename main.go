package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/olekukonko/tablewriter"
)

type feedItem struct {
	Feed string
	Last string
}

type config struct {
	Exec  string
	Feeds []feedItem
}

func main() {
	home, ok := os.LookupEnv("HOME")
	if !ok {
		panic(errors.New("HOME is not set"))
	}

	xdgConfigHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		xdgConfigHome = fmt.Sprintf("%v/.config", home)
	}

	var config string
	flag.StringVar(&config, "config", fmt.Sprintf("%v/rssd/config.json", xdgConfigHome), "path to config file")
	flag.Parse()

	err := initConfig(config)
	if err != nil {
		panic(err.Error())
	}

	if len(flag.Args()) == 0 {
		synchronize(config)
	}

	if flag.Arg(0) == "add-feed" {
		if len(flag.Args()) < 2 {
			fmt.Fprintln(os.Stderr, "insufficient number of arguments")
			os.Exit(2)
		}
		err := addFeed(config, flag.Arg(1))
		if err != nil {
			panic(err.Error())
		}
		os.Exit(0)
	}

	if flag.Arg(0) == "list-feed" {
		err := listFeed(config)
		if err != nil {
			panic(err.Error())
		}
		os.Exit(0)
	}

	if flag.Arg(0) == "set-exec" {
		if len(flag.Args()) < 2 {
			fmt.Fprintln(os.Stderr, "insufficient number of arguments")
			os.Exit(2)
		}
		err := setExec(config, flag.Arg(1))
		if err != nil {
			panic(err.Error())
		}
		os.Exit(0)
	}

	os.Exit(1)
}

func synchronize(p string) {
	c, err := readConfig(p)
	if err != nil {
		panic(err)
	}

	for i, v := range c.Feeds {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		defer cancel()

		f, err := (gofeed.NewParser()).ParseURLWithContext(v.Feed, ctx)
		if err != nil {
			panic(err)
		}

		if f.Items[0].Link != v.Last {
			s := c.Exec
			for q, r := range map[string]string{
				"&title":            f.Title,
				"&desc":             f.Description,
				"&lang":             f.Language,
				"&item_title":       f.Items[0].Title,
				"&item_link":        f.Items[0].Link,
				"&item_pubDate":     f.Items[0].Published,
				"&item_desc":        f.Items[0].Description,
				"&item_authorName":  f.Items[0].Author.Name,
				"&item_authorEmail": f.Items[0].Author.Email,
			} {
				s = strings.ReplaceAll(s, q, r)
				s = os.ExpandEnv(s)
			}

			err = exec.Command("sh", "-c", s).Run()
			if err != nil {
				panic(err)
			}

			v.Last = f.Items[0].Link
			c.Feeds[i] = v
		}
	}

	err = writeConfig(p, c)
	if err != nil {
		panic(err)
	}

	os.Exit(0)
}

func setExec(p string, e string) error {
	c, err := readConfig(p)
	if err != nil {
		return err
	}

	c.Exec = e

	err = writeConfig(p, c)
	if err != nil {
		return err
	}

	return nil
}

func listFeed(p string) error {
	s, err := readConfig(p)
	if err != nil {
		return err
	}

	t := tablewriter.NewWriter(os.Stdout)
	t.SetHeader([]string{"Feed", "Last"})

	for _, v := range s.Feeds {
		t.Append([]string{v.Feed, v.Last})
	}

	t.Render()

	return nil
}

func addFeed(p string, feed string) error {
	s, err := readConfig(p)
	if err != nil {
		return err
	}

	flag := false
	for _, v := range s.Feeds {
		if v.Feed == feed {
			flag = true
		}
	}
	if flag {
		return errors.New("duplicate feed")
	}

	s.Feeds = append(s.Feeds, feedItem{feed, ""})

	err = writeConfig(p, s)
	if err != nil {
		return err
	}

	return nil
}

func readConfig(p string) (*config, error) {
	f, err := os.OpenFile(p, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	d := make([]byte, 1000)
	_, err = f.Read(d)
	if err != nil {
		return nil, err
	}

	d = bytes.Trim(d, "\x00")

	var s config
	err = json.Unmarshal(d, &s)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func writeConfig(p string, c *config) error {
	f, err := os.OpenFile(p, os.O_RDWR, 0600)
	if err != nil {
		return err
	}

	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	err = f.Truncate(0)
	if err != nil {
		return err
	}

	_, err = f.Write(b)
	if err != nil {
		return err
	}

	return nil
}

func initConfig(p string) error {
	_, err := os.Stat(p)
	if os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(p), 0755)
		if err != nil {
			return err
		}

		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return err
		}

		c, err := json.Marshal(config{})
		if err != nil {
			return err
		}

		f.Truncate(0)
		_, err = f.Write(c)
		if err != nil {
			return err
		}
	}

	return nil
}