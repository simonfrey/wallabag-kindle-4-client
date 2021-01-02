package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/simonfrey/wallabago"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
)

var CLI struct {
	Username     string `kong:"arg,required,name='username',help='Your wallabag username'"`
	Password     string `kong:"arg,required,name='password',help='Your wallabag password'"`
	ClientID     string `kong:"arg,required,name='client-id',help='Your wallabag client id'"`
	ClientSecret string `kong:"arg,required,name='client-secret',help='Your wallabag client secret'"`

	EbookBasePath string `kong:"default='/mnt/base-us/documents',name='path',help='Base path where to place the ebooks',type='path'"`
	WallabagUrl   string `kong:"default='https://app.wallabag.it',name='wallabag-server',help='Url of your wallabag server'"`
	ReloadCommand string `kong:"default='dbus-send --system /default com.lab126.powerd.resuming int32:1',name='reload-command',help='Command to tell kindle to reload ebooks'"`
	SkipTLS       bool   `kong:"default='false',name='skip-tls',help='Skip tls check. Good for using wallabag with ip'"`
}

var exists interface{}
var fileFmt = "wb_%d.mobi"

func main() {
	// Load command parameters
	kong.Parse(&CLI)
	apiURL := CLI.WallabagUrl + "/api"

	// Setup wallabag client
	wallabago.SetConfig(wallabago.NewWallabagConfig(CLI.WallabagUrl, CLI.ClientID, CLI.ClientSecret, CLI.Username, CLI.Password))

	// Set wallabag http client
	if CLI.SkipTLS {
		wallabago.HttpClient = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	}
	//
	// Check for currently existing files

	// Load ids from ids.wb file
	ids := []int{}
	idsData, err := ioutil.ReadFile(path.Join(CLI.EbookBasePath, "ids.wb"))
	if err == nil {
		err = json.Unmarshal(idsData, &ids)
		if err != nil {
			log.Fatal(errors.Wrap(err, "Could not unmarshal ids"))
		}
	}

	// Check if the files with the ids from ids.wb exist
	ebookExists := make(map[int]bool, len(ids))
	for _, id := range ids {
		if _, err := os.Stat(path.Join(CLI.EbookBasePath, fmt.Sprintf(fileFmt, id))); err == nil {
			ebookExists[id] = true
			continue
		}
		ebookExists[id] = false
	}

	// Iterate over existing info and check if we need to archive any
	for id, fileExists := range ebookExists {
		if fileExists {
			// Nothing to do
			continue
		}

		// An id exists in id file but not on filesystem. The ebook was deleted and we want
		// to archive the entry in wallabag

		log.Println("ARCHIVE: ", id)

		archiveJson, err := json.Marshal(struct {
			Archive int `json:"archive"`
		}{Archive: 1})
		if err != nil {
			log.Fatal(errors.Wrap(err, "Could not marshal archive body"))
		}
		// File does not exists anymore. Archive article
		archiveUrl := fmt.Sprintf("%s/entries/%d.json", apiURL, id)
		_, err = wallabago.APICall(archiveUrl, "PATCH", archiveJson)
		if err != nil {
			log.Fatal(errors.Wrap(err, "could not execute archive call"))
		}

	}

	// Load all currently active entries from wallabag
	f := false
	entries, err := wallabago.GetAllEntriesFiltered(&f, nil)
	if err != nil {
		log.Fatal(errors.Wrap(err, "could not get all non-archived entries"))
	}

	shouldExistsIDsMap := map[int]interface{}{}
	shouldExistsIDs := []int{}

	// Iterate over all entries and download the .mobi files
	for _, entry := range entries {
		// Add id to exist map
		shouldExistsIDsMap[entry.ID] = exists
		shouldExistsIDs = append(shouldExistsIDs, entry.ID)

		// Check if id already exists
		if _, idExists := ebookExists[entry.ID]; idExists {
			// Entry exists. Skip download
			continue
		}

		// Download mobi file
		log.Printf("Download %d", entry.ID)

		mobiURL := fmt.Sprintf("%s/entries/%d/export.mobi", apiURL, entry.ID)
		mobiData, err := wallabago.APICall(mobiURL, "GET", nil)
		if err != nil {
			log.Fatal(errors.Wrapf(err, "could not download mobi data for %d", entry.ID))
		}

		err = ioutil.WriteFile(path.Join(CLI.EbookBasePath, fmt.Sprintf(fileFmt, entry.ID)), mobiData, 0644)
		if err != nil {
			log.Fatal(errors.Wrapf(err, "could not create mobi file for %d", entry.ID))
		}

		// Write current version of shouldExistIDs
		idsJson, err := json.Marshal(shouldExistsIDs)
		if err != nil {
			log.Fatal(errors.Wrap(err, "could not marshal shouldExistsIDs"))
		}
		err = ioutil.WriteFile(path.Join(CLI.EbookBasePath, "ids.wb"), idsJson, 0644)
		if err != nil {
			log.Fatal(errors.Wrap(err, "could not write ids.wb"))
		}
	}

	// Check if there are files, that are not meant to be in the filesystem anymore (as the entries where archived
	// on the wallabag server)
	for id, fileExists := range ebookExists {
		if !fileExists {
			// File does not exits. Nothing to do
			continue
		}
		if _, shouldExists := shouldExistsIDsMap[id]; shouldExists {
			// File should exists. Nothing to do
			continue
		}

		// File should not exists. Delete it
		err := os.Remove(path.Join(CLI.EbookBasePath, fmt.Sprintf(fileFmt, id)))
		if err != nil {
			log.Fatal(errors.Wrapf(err, "could not remove file for %d", id))
		}
	}

	// Signal kindle to re-read the ebook
	if commandToExecute := strings.Split(strings.TrimSpace(CLI.ReloadCommand), " "); len(commandToExecute) > 0 {
		cmd := exec.Command(commandToExecute[0], commandToExecute[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatal(errors.Wrapf(err, "could not execute reload command: %q", string(out)))
		}
	}

	log.Println("Done")
}
