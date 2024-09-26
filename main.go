package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const userAgent = "My humble asset manager / browser"

var (
	httpClient http.Client
)

func main() {
	httpClient.Timeout = time.Hour
	client, err := NewClientWithResponses("https://api.polyhaven.com", WithHTTPClient(&httpClient), WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("User-Agent", userAgent)
		return nil
	}))
	if err != nil {
		panic(err)
	}

	for _, asset_type := range []string{"models", "textures"} {
		time.Sleep(time.Millisecond * 321)
		dir_path := "/home/_/gda/polyhaven_" + asset_type
		dir_path_tags, dir_path_cats := filepath.Join(dir_path, "_by_tag"), filepath.Join(dir_path, "_by_cat")
		for _, dir_path := range []string{dir_path, dir_path_cats, dir_path_tags} {
			if err := os.MkdirAll(dir_path, os.ModePerm); err != nil {
				panic(err)
			}
		}

		resp_assets, err := client.GetAssetsWithResponse(context.Background(), &GetAssetsParams{Type: &asset_type})
		if err != nil {
			panic(err)
		}
		if status_code := resp_assets.StatusCode(); status_code != 200 {
			panic(status_code)
		}
		assets := map[string]any{}
		if err := json.Unmarshal(resp_assets.Body, &assets); err != nil {
			panic(err)
		}
		for asset_id := range assets {
			time.Sleep(time.Millisecond * 321)
			resp_files, err := client.GetFilesIdWithResponse(context.Background(), asset_id)
			if err != nil {
				panic(err)
			}
			if status_code := resp_files.StatusCode(); status_code != 200 {
				panic(status_code)
			}
			files := map[string] /*format*/ map[string] /*resolution*/ FileInfo{}
			if err := json.Unmarshal(resp_files.Body, &files); err != nil {
				panic(err)
			}
			println(asset_type, "\t", asset_id)
			var had_gltf, had_fbx bool
			for format_id := range files {
				had_fbx = had_fbx || (format_id == "fbx")
				if had_gltf = (format_id == "gltf"); had_gltf {
					break
				}
			}

			nope := If((asset_type != "models"), (!had_gltf), (!had_gltf) && !had_fbx)
			if nope {
				continue
			}

			dir_path_asset := filepath.Join(dir_path, asset_id)
			if err := os.MkdirAll(dir_path_asset, os.ModePerm); err != nil {
				panic(err)
			}

			for _, this_cat := range assets[asset_id].(map[string]any)["categories"].([]any) {
				cat := this_cat.(string)
				dir_path_cat := filepath.Join(dir_path_cats, cat)
				if err := os.MkdirAll(dir_path_cat, os.ModePerm); err != nil {
					panic(err)
				}
				if err := os.Symlink(dir_path_asset, filepath.Join(dir_path_cat, asset_id)); (err != nil) && (!os.IsExist(err)) && (err != os.ErrExist) {
					panic(err)
				}
			}
			for _, this_tag := range assets[asset_id].(map[string]any)["tags"].([]any) {
				tag := this_tag.(string)
				dir_path_tag := filepath.Join(dir_path_tags, tag)
				if err := os.MkdirAll(dir_path_tag, os.ModePerm); err != nil {
					panic(err)
				}
				if err := os.Symlink(dir_path_asset, filepath.Join(dir_path_tag, asset_id)); (err != nil) && (!os.IsExist(err)) && (err != os.ErrExist) {
					panic(err)
				}
			}

			for resolution, file_info := range files[If(had_gltf, "gltf", "fbx")] {
				dir_path_asset_res := filepath.Join(dir_path_asset, resolution)
				if err := os.MkdirAll(dir_path_asset_res, os.ModePerm); err != nil {
					panic(err)
				}
				file := If(file_info.Gltf == nil, file_info.Fbx, file_info.Gltf)
				println("\t", resolution, "\t", filepath.Base(file.Url), len(file.Include))
				if err := downloadFileTo(file.Url, filepath.Join(dir_path_asset_res, filepath.Base(file.Url)), file.Md5); err != nil {
					panic(err)
				}
				for file_path_rel, file := range file.Include {
					println("\t\t", file_path_rel)
					if dp := filepath.Dir(file_path_rel); (dp != "") && (dp != ".") {
						if err := os.MkdirAll(filepath.Join(dir_path_asset_res, dp), os.ModePerm); err != nil {
							panic(err)
						}
					}
					if err := downloadFileTo(file.Url, filepath.Join(dir_path_asset_res, file_path_rel), file.Md5); err != nil {
						panic(err)
					}
				}
			}
		}
	}
}

type FileInfo struct {
	Gltf *fileInfo `json:"gltf"`
	Fbx  *fileInfo `json:"fbx"`
}

type fileInfo struct {
	Url     string              `json:"url"`
	Md5     string              `json:"md5"`
	Size    int                 `json:"size"`
	Include map[string]fileInfo `json:"include"`
}

func If[T any](b bool, t T, f T) T {
	if b {
		return t
	}
	return f
}

func downloadFileTo(srcUrl string, dstFilePath string, md5Hash string) error {
	time.Sleep(time.Millisecond * 321)
	data_local, _ := os.ReadFile(dstFilePath)
	if len(data_local) > 0 {
		if hash_local := hashMd5(data_local); hash_local == md5Hash {
			return nil
		}
	}

	req, err := http.NewRequest("GET", srcUrl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data_remote, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	hash_local := hashMd5(data_remote)
	if hash_local != md5Hash {
		return (fmt.Errorf("hash mismatch: wanted %s but got %s", md5Hash, hash_local))
	}
	if err = os.WriteFile(dstFilePath, data_remote, os.ModePerm); err != nil {
		return err
	}

	return nil
}

func hashMd5(data []byte) string {
	md5 := md5.New()
	if _, err := md5.Write(data); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", md5.Sum(nil))
}
