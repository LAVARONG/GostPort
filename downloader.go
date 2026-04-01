package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// EnsureGost 检测本地是否有 gost，如果没有则自动从 GitHub 下载最新 Releases
func EnsureGost() (string, error) {
	// 1. 优先查同级目录
	exePath, err := os.Executable()
	if err != nil {
		return "gost", err
	}
	dir := filepath.Dir(exePath)
	binName := "gost"
	if runtime.GOOS == "windows" {
		binName = "gost.exe"
	}
	localBin := filepath.Join(dir, binName)

	if _, err := os.Stat(localBin); err == nil {
		return localBin, nil
	}

	// 2. 查环境变量 PATH
	if path, err := exec.LookPath(binName); err == nil {
		log.Printf("在系统 PATH 中发现 gost 环境: %s", path)
		return path, nil
	}

	// 3. 自动向 Github Releases 请求最新版
	log.Printf("未发现 %s，正在从 Github 智能抓取并解压适配本架构(%s)的预编译包...", binName, runtime.GOARCH)
	err = DownloadGost(dir)
	if err != nil {
		return binName, fmt.Errorf("全自动下载及配置失败: %w", err)
	}

	if _, err := os.Stat(localBin); err == nil {
		return localBin, nil
	}
	return binName, fmt.Errorf("下载提取之后依然未发现 %s", binName)
}

// DownloadGost 执行下载、智能识别结构并解压缩行为
func DownloadGost(destDir string) error {
	api := "https://api.github.com/repos/go-gost/gost/releases/latest"
	resp, err := http.Get(api)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("github api err: %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	// 版本标签如 "v3.2.6" 洗去 "v"
	tag := strings.TrimPrefix(result.TagName, "v")

	// 根据 go-gost release 的命名规则： gost_3.2.6_windows_amd64.zip
	targetName := fmt.Sprintf("gost_%s_%s_%s", tag, runtime.GOOS, runtime.GOARCH)

	var downloadURL string
	var filename string
	for _, a := range result.Assets {
		if strings.HasPrefix(a.Name, targetName) {
			downloadURL = a.BrowserDownloadURL
			filename = a.Name
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("发行库中未找到您的系统和架构编译包: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	log.Printf("🎯精确定位到目标版本: %s", filename)
	tmpFile := filepath.Join(destDir, filename)

	out, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	req, _ := http.NewRequest("GET", downloadURL, nil)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		out.Close()
		return err
	}

	_, err = io.Copy(out, r.Body)
	r.Body.Close()
	out.Close()
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	log.Printf("📦下载完成，准备自解压...")

	if strings.HasSuffix(filename, ".zip") {
		return extractZip(tmpFile, destDir)
	} else if strings.HasSuffix(filename, ".tar.gz") {
		return extractTarGz(tmpFile, destDir)
	}
	return fmt.Errorf("无法处理此文件的压缩格式")
}

// extractZip ZIP流媒体解压处理器
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "gost.exe" || f.Name == "gost" {
			outPath := filepath.Join(destDir, f.Name)
			rc, err := f.Open()
			if err != nil {
				return err
			}
			out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				rc.Close()
				return err
			}
			_, err = io.Copy(out, rc)
			out.Close()
			rc.Close()
			if err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("脱壳未发现合法内核代码文件")
}

// extractTarGz GZ格式流解压处理器
func extractTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzf, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzf.Close()

	tarReader := tar.NewReader(gzf)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// 有些打包文件里包含了子目录，这里通过 HasSuffix 宽松匹配
		if header.Typeflag == tar.TypeReg && (strings.HasSuffix(header.Name, "gost") || strings.HasSuffix(header.Name, "gost.exe")) {
			// 将实际写出的文件名写死在当前根目录，丢弃包内文件夹环境
			binName := "gost"
			if runtime.GOOS == "windows" {
				binName = "gost.exe"
			}
			outPath := filepath.Join(destDir, binName)
			out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tarReader)
			out.Close()
			if err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("解压流程中异常未捕获匹配内核")
}
