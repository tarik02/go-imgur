package imgur

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
)

// UploadImage uploads the image to imgur
// image                Can be a binary file, base64 data, or a URL for an image. (up to 10MB)
// album       optional The id of the album you want to add the image to.
//
//	For anonymous albums, album should be the deletehash that is returned at creation.
//
// dtype                The type of the file that's being sent; file, base64 or URL
// title       optional The title of the image.
// description optional The description of the image.
// returns image info, status code of the upload, error
func (client *Client) UploadImage(image []byte, album string, dtype string, title string, description string) (*ImageInfo, int, error) {
	if image == nil {
		return nil, -1, errors.New("Invalid image")
	}
	if dtype != "file" && dtype != "base64" && dtype != "URL" {
		return nil, -1, errors.New("Passed invalid dtype: " + dtype + ". Please use file/base64/URL.")
	}

	reqbody := &bytes.Buffer{}
	writer := multipart.NewWriter(reqbody)
	createUploadForm(writer, image, album, dtype, title, description)
	writer.Close()

	URL := client.createAPIURL("image")
	req, err := http.NewRequest("POST", URL, reqbody)
	client.Log.Debugf("Posting to URL %v\n", URL)
	if err != nil {
		return nil, -1, errors.New("Could create request for " + URL + " - " + err.Error())
	}

	req.Header.Add("Authorization", "Client-ID "+client.imgurAccount.clientID)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	if client.rapidAPIKey != "" {
		req.Header.Add("X-RapidAPI-Key", client.rapidAPIKey)
	}

	res, err := client.httpClient.Do(req)
	if err != nil {
		return nil, -1, errors.New("Could not post " + URL + " - " + err.Error())
	}
	defer res.Body.Close()

	// Read the whole body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, -1, errors.New("Problem reading the body of " + URL + " - " + err.Error())
	}

	// client.Log.Debugf("%v\n", string(body[:]))

	dec := json.NewDecoder(bytes.NewReader(body))
	var img imageInfoDataWrapper
	if err = dec.Decode(&img); err != nil {
		return nil, -1, errors.New("Problem decoding json result from image upload - " + err.Error() + ". JSON(?): " + string(body))
	}

	if !img.Success {
		return nil, img.Status, errors.New("Upload to imgur failed with status: " + strconv.Itoa(img.Status))
	}

	img.Ii.Limit, _ = extractRateLimits(res.Header)

	return img.Ii, img.Status, nil
}

func createUploadForm(writer *multipart.Writer, image []byte, album string, dtype string, title string, description string) {
	part, _ := writer.CreateFormFile("image", "image")
	_, _ = part.Write(image)

	_ = writer.WriteField("image", string(image[:]))
	_ = writer.WriteField("type", dtype)

	if album != "" {
		_ = writer.WriteField("album", album)
	}
	if title != "" {
		_ = writer.WriteField("title", title)
	}
	if description != "" {
		_ = writer.WriteField("description", description)
	}
}

// UploadImageFromFile uploads a file given by the filename string to imgur.
func (client *Client) UploadImageFromFile(filename string, album string, title string, description string) (*ImageInfo, int, error) {
	client.Log.Infof("*** IMAGE UPLOAD ***\n")
	f, err := os.Open(filename)
	if err != nil {
		return nil, 500, fmt.Errorf("Could not open file %v - Error: %v", filename, err)
	}
	defer f.Close()
	fileinfo, err := f.Stat()
	if err != nil {
		return nil, 500, fmt.Errorf("Could not stat file %v - Error: %v", filename, err)
	}
	size := fileinfo.Size()
	b := make([]byte, size)
	n, err := f.Read(b)
	if err != nil || int64(n) != size {
		return nil, 500, fmt.Errorf("Could not read file %v - Error: %v", filename, err)
	}

	return client.UploadImage(b, album, "file", title, description)
}
