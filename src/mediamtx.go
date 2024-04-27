package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

func downloadMediaMTX() {
	os.Mkdir("mediamtx", 0755)
	os.Chdir("mediamtx")

	fmt.Println("Downloading MediaMTX...")
	url := generateDownloadUrl()

	// Download MediaMTX
	res, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
	}

	// Decompress MediaMTX
	if filepath.Ext(url) == ".zip" {
		unZip(body)
	} else if filepath.Ext(url) == ".gz" {
		unTarGz(body)
	}

	fmt.Println("MediaMTX downloaded.")
	os.Chdir("..")
}

func unZip(body []byte) {
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		fmt.Println(err)
	}

	for _, zipFile := range zipReader.File {
		f, err := zipFile.Open()
		if err != nil {
			fmt.Println(err)
		}
		defer f.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(f)
		createWriteFile(zipFile.Name, buf.Bytes())
	}
}

func unTarGz(body []byte) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		fmt.Println(err)
	}

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
		}

		buf := new(bytes.Buffer)
		buf.ReadFrom(tarReader)
		createWriteFile(header.Name, buf.Bytes())
	}
	os.Chmod("mediamtx", 0755)

	gzipReader.Close()
}

func createWriteFile(name string, body []byte) {
	file, err := os.Create(name)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	_, err = file.Write(body)
	if err != nil {
		fmt.Println(err)
	}
}

type Release struct {
	TagName string `json:"tag_name"`
}

func generateDownloadUrl() string {
	const owner = "bluenviron"
	const repo = "mediamtx"

	// Fetch latest release
	githubReleasesApiUrl, err := url.JoinPath("https://api.github.com/repos/", owner, "/", repo, "/releases/latest")
	if err != nil {
		fmt.Println(err)
	}

	res, err := http.Get(githubReleasesApiUrl)
	if err != nil {
		fmt.Println(err)
	}
	defer res.Body.Close()

	var release Release
	err = json.NewDecoder(res.Body).Decode(&release)
	if err != nil {
		fmt.Println(err)
	}

	downloadUrlBase, err := url.JoinPath("https://github.com/", owner, repo, "/releases/latest/download")
	if err != nil {
		fmt.Println(err)
	}

	downloadUrlSuffix := generateSuffixUrl()
	downloadPackageUrl := repo + "_" + release.TagName + "_" + downloadUrlSuffix

	downloadUrl, err := url.JoinPath(downloadUrlBase, downloadPackageUrl)
	if err != nil {
		fmt.Println(err)
	}

	return downloadUrl
}

func generateSuffixUrl() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	if os == "windows" && arch == "amd64" {
		return "windows_amd64.zip"
	} else if os == "darwin" && arch == "amd64" {
		return "darwin_amd64.tar.gz"
	} else if os == "linux" && arch == "arm64" {
		return "linux_arm64.tar.gz"
	} else if os == "linux" && arch == "amd64" {
		return "linux_amd64.tar.gz"
	} else {
		return ""
	}
}

func launchMediaMTX() {
	fmt.Println("Launching MediaMTX...")

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("./mediamtx.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	} else {
		cmd = exec.Command("./mediamtx")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "mediamtx"

	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("MediaMTX launched.")
}
