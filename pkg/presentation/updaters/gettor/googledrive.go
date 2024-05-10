// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gettor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/resources"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func newGoogleDriveUpdater(cfg *internal.GoogleDriveUpdater) (provider, error) {
	updater := googleDriveUpdater{config: cfg, ctx: context.Background()}
	var err error
	updater.drive, err = updater.createApiClientFromConfig()
	return &updater, err
}

type googleDriveUpdater struct {
	ctx    context.Context
	config *internal.GoogleDriveUpdater
	drive  *drive.Service
}

func (g googleDriveUpdater) needsUpdate(platform string, version resources.Version) bool {
	folders, err := g.getPlatformFolders(platform)
	if err != nil {
		log.Println("[Google Drive] unable to check for update", err)
		return false
	}
	if len(folders) == 0 {
		log.Println("[Google Drive] needs update for", platform)
		return true
	}

	for _, folder := range folders {
		parts := strings.Split(folder.Name, "-")
		if len(parts) != 2 {
			continue
		}

		releaseVersion, err := resources.Str2Version(parts[1])
		if err != nil {
			continue
		}

		if version.Compare(releaseVersion) == 1 {
			log.Println("[Google Drive] needs update for", platform)
			return true
		}
	}

	return false
}

func (g googleDriveUpdater) newRelease(platform string, version resources.Version) uploadFileFunc {
	oldFolders, err := g.getPlatformFolders(platform)
	if err != nil {
		log.Println("[Google Drive] unable to get platform folders", err)
		return nil
	}

	folderName := fmt.Sprintf("%s-%s", platform, version.String())
	folderID, err := g.mkdir(folderName)
	if err != nil {
		log.Println("[Google Drive] can't create folder", folderName, err)
		return nil
	}

	for _, folder := range oldFolders {
		err := g.rmdir(folder)
		if err != nil {
			log.Println("[Google Drive] Error deleting a folder", folder.Name, ":", err)
		}
	}

	return func(binaryPath string, sigPath string) *resources.TBLink {
		link := resources.NewTBLink()

		{
			var err error
			link.Link, err = g.createLinkFromPath(folderID, binaryPath)
			if err != nil {
				log.Println("[Google Drive] Unable to create link for binary ", err)
				return nil
			}
		}
		{
			var err error
			link.SigLink, err = g.createLinkFromPath(folderID, sigPath)
			if err != nil {
				log.Println("[Google Drive] Unable to create link for binary ", err)
				return nil
			}
		}

		link.Version = version
		link.Provider = "Google Drive"
		link.Platform = platform
		link.FileName = path.Base(binaryPath)
		return link
	}

}

func (g googleDriveUpdater) createLinkFromPath(folderID string, filePath string) (string, error) {
	filename := path.Base(filePath)
	fd, err := os.Open(filePath)
	if err != nil {
		log.Println("[Google Drive] Unable to create file to be uploaded", err)
		return "", err
	}
	defer fd.Close()

	downloadLink, err := g.uploadFileAndGetLink(folderID, filename, fd)
	if err != nil {
		log.Println("[Google Drive] Unable to get file link ", err)
		return "", err
	}
	return downloadLink, nil
}

// tokenFromFile Retrieves a token from a local file.
// reused from https://developers.google.com/drive/api/v3/quickstart/go
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// getTokenFromWeb Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config, authCode string) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Printf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func (g googleDriveUpdater) createApiClientFromConfig() (*drive.Service, error) {
	b, err := os.ReadFile(g.config.AppCredentialPath)
	if err != nil {
		return nil, err
	}
	config, err := google.ConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		return nil, err
	}

	userToken, err := tokenFromFile(g.config.UserCredentialPath)
	if err != nil {
		return nil, err
	}

	client := config.Client(g.ctx, userToken)
	srv, err := drive.NewService(g.ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func (g googleDriveUpdater) getPlatformFolders(platform string) ([]*drive.File, error) {
	query := fmt.Sprintf("'%v' in parents and name contains '%v' and mimeType = 'application/vnd.google-apps.folder'", g.config.ParentFolderID, platform)
	fileList, err := g.drive.Files.List().Q(query).Do()
	if err != nil {
		return nil, err
	}
	return fileList.Files, nil
}

func (g googleDriveUpdater) mkdir(folderName string) (folderID string, err error) {
	file := &drive.File{
		Name:     folderName,
		Parents:  []string{g.config.ParentFolderID},
		MimeType: "application/vnd.google-apps.folder",
	}
	folder, err := g.drive.Files.Create(file).Do()
	if err != nil {
		return "", err
	}
	return folder.Id, nil
}

func (g googleDriveUpdater) rmdir(file *drive.File) error {
	return g.drive.Files.Delete(file.Id).Do()
}

func (g googleDriveUpdater) uploadFileAndGetLink(folderID string, filename string, reader io.Reader) (string, error) {
	file := &drive.File{Name: filename, Parents: []string{folderID}}
	result, err := g.drive.Files.Create(file).Media(reader).Do()
	if err != nil {
		return "", err
	}

	_, err = g.drive.Permissions.Create(result.Id, &drive.Permission{Type: "anyone", Role: "reader"}).Do()
	if err != nil {
		return "", err
	}

	getResult, err := g.drive.Files.Get(result.Id).Fields("webContentLink").Do()
	if err != nil {
		return "", err
	}

	return getResult.WebContentLink, err
}

func (g googleDriveUpdater) createToken(authCode string) error {
	b, err := os.ReadFile(g.config.AppCredentialPath)
	if err != nil {
		return err
	}
	config, err := google.ConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		return err
	}
	token := getTokenFromWeb(config, authCode)
	if token == nil {
		return errors.New("unable to create token")
	}
	saveToken(g.config.UserCredentialPath, token)
	return nil
}
