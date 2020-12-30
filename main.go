package main

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/simonfrey/wallabago"
	"io/ioutil"
	"log"
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
}

func main() {
	kong.Parse(&CLI)

	apiURL := CLI.WallabagUrl + "/api"

	wallabago.SetConfig(wallabago.NewWallabagConfig(CLI.WallabagUrl, CLI.ClientID, CLI.ClientSecret, CLI.Username, CLI.Password))

	fileFmt := "wb_%d.mobi"

	ids := []int{}

	idsData, err := ioutil.ReadFile(path.Join(CLI.EbookBasePath, "ids.wb"))
	if err == nil {

		err = json.Unmarshal(idsData, &ids)
		if err != nil {
			log.Fatal(err)
		}
	}

	ebookExists := make(map[int]bool, len(ids))

	for _, id := range ids {
		if _, err := os.Stat(path.Join(CLI.EbookBasePath, fmt.Sprintf(fileFmt, id))); err == nil {
			ebookExists[id] = true
			continue
		}
		ebookExists[id] = false
	}

	existingIds := []int{}
	for id, fileExists := range ebookExists {
		if fileExists {
			// Nothing to do
			existingIds = append(existingIds, id)
			continue
		}

		fmt.Println("ARCHIVE: ", id)

		archiveJson, err := json.Marshal(struct {
			Archive int `json:"archive"`
		}{Archive: 1})
		if err != nil {
			log.Fatal(err)
		}
		// File does not exists anymore. Archive article
		archiveUrl := fmt.Sprintf("%s/entries/%d.json", apiURL, id)
		_, err = wallabago.APICall(archiveUrl, "PATCH", archiveJson)
		if err != nil {
			log.Fatal(err)
		}

	}
	idsJson, err := json.Marshal(existingIds)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(path.Join(CLI.EbookBasePath, "ids.wb"), idsJson, 0644)
	if err != nil {
		log.Fatal(err)
	}

	entries, err := getNonArchivedEntries()
	if err != nil {
		log.Fatal(err)
	}

	var exists interface{}

	shouldExistsIDsMap := map[int]interface{}{}
	shouldExistsIDs := []int{}

	for _, entry := range entries {
		mobiURL := fmt.Sprintf("%s/entries/%d/export.mobi", apiURL, entry.ID)

		mobiData, err := wallabago.APICall(mobiURL, "GET", nil)
		if err != nil {
			log.Fatal(err)
		}

		// Check if id already exists
		if _, idExists := ebookExists[entry.ID]; !idExists {
			fmt.Println("Create new file: ", entry.ID)
			err = ioutil.WriteFile(path.Join(CLI.EbookBasePath, fmt.Sprintf(fileFmt, entry.ID)), mobiData, 0644)
			if err != nil {
				log.Fatal(err)
			}

		}
		shouldExistsIDsMap[entry.ID] = exists
		shouldExistsIDs = append(shouldExistsIDs, entry.ID)

		idsJson, err := json.Marshal(shouldExistsIDs)
		if err != nil {
			log.Fatal(err)
		}
		err = ioutil.WriteFile(path.Join(CLI.EbookBasePath, "ids.wb"), idsJson, 0644)
		if err != nil {
			log.Fatal(err)
		}
	}

	for id, fileExists := range ebookExists {
		if !fileExists {
			// File does not exits anyways
			continue
		}
		if _, shouldExists := shouldExistsIDsMap[id]; shouldExists {
			// File should exists. Do nothing
			continue
		}

		// File should not exists. Delete it
		err := os.Remove(path.Join(CLI.EbookBasePath, fmt.Sprintf(fileFmt, id)))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Signal kindle to re-read the ebooks
	commandToExecute := strings.Split(CLI.ReloadCommand, " ")
	cmd := exec.Command(commandToExecute[0], commandToExecute[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}

func getNonArchivedEntries() ([]wallabago.Item, error) {
	page := -1
	perPage := -1
	e, err := wallabago.GetEntries(wallabago.APICall, 0, -1, "", "", page, perPage, "")
	if err != nil {
		log.Println("GetAllEntries: first GetEntries call failed", err)
		return nil, err
	}
	allEntries := e.Embedded.Items
	if e.Total > len(allEntries) {
		secondPage := e.Page + 1
		perPage = e.Limit
		pages := e.Pages
		for i := secondPage; i <= pages; i++ {
			e, err := wallabago.GetEntries(wallabago.APICall, -1, -1, "", "", i, perPage, "")
			if err != nil {
				log.Printf("GetAllEntries: GetEntries for page %d failed: %v", i, err)
				return nil, err
			}
			tmpAllEntries := e.Embedded.Items
			allEntries = append(allEntries, tmpAllEntries...)
		}
	}
	return allEntries, err
}
