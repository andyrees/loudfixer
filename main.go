/*
Copyright 2014 Andrew Rees.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
either express or implied. See the License for the specific
language governing permissions and limitations under the
License.
*/

package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type media struct {
	streams []struct {
		codecName     string  `json:"codec_name"`
		bitRate       string  `json:"bit_rate"`
		durationTs    float64 `json:"duration_ts"`
		codecType     string  `json:"codec_type"`
		codecTimeBase string  `json:"codec_time_base"`
		profile       string  `json:"Profile"`
		tags          struct {
			language         string `json:"language"`
			majorBrand       string `json:"major_brand"`
			minorVersion     string `json:"minor_version"`
			compatibleBrands string `json:"compatible_brands"`
			handlerName      string `json:"handler_name"`
		}
		duration           string  `json:"duration"`
		index              float64 `json:"index"`
		sampleAspectRatio  string  `json:"sample_aspect_ratio"`
		codecTag           string  `json:"codec_tag"`
		codecTagString     string  `json:"codec_tag_string"`
		startPts           float64 `json:"start_pts"`
		sampleRate         string  `json:"sample_rate"`
		nbFrames           string  `json:"nb_frames"`
		avgFrameRate       string  `json:"avg_frame_rate"`
		codecLongName      string  `json:"codec_long_name"`
		displayAspectRatio string  `json:"display_aspect_ratio"`
		level              float64 `json:"level"`
		hasBFrames         float64 `json:"has_b_frames"`
		pixFmt             string  `json:"pix_fmt"`
		timeBase           string  `json:"time_base"`
		height             float64 `json:"height"`
		bitsPerSample      float64 `json:"bits_per_sample"`
		width              float64 `json:"width"`
		sampleFmt          string  `json:"sample_fmt"`
		startTime          string  `json:"start_time"`
		channels           float64 `json:"channels"`
		rFrameRate         string  `json:"r_frame_rate"`
	}
	format struct {
		formatName     string `json:"format_name"`
		formatLongName string `json:"ormat_long_name"`
		nbStreams      int    `json:"nb_streams"`
		duration       string `json:"duration"`
		startTime      string `json:"start_time"`
		size           string `json:"size"`
		filename       string `json:"filename"`
		bitRate        string `json:"filename"`
		tags           struct {
			language         string `json:"language"`
			majorBrand       string `json:"major_brand"`
			minorVersion     string `json:"minor_version"`
			compatibleBrands string `json:"compatible_brands"`
			handlerName      string `json:"handler_name"`
		}
	}
}

func fcheck(filepath string) error {
	_, err := os.Stat(filepath)
	if err != nil {
		return err
	}
	return nil
}

func getDataFromFfprobe(filepath string) (string, error) {
	ffprobePath, lookerr := exec.LookPath("ffprobe")
	if lookerr != nil {
		return "", lookerr
	}

	jsonResultBytes, err := exec.Command(ffprobePath, "-show_format", "-show_streams", "-print_format", "json=c=1", filepath).Output()
	if err != nil {
		return "", err
	}

	return string(jsonResultBytes), nil
}

type mediaFileLoudness struct {
	fileName                    string
	passedOrFailed              bool
	loudness                    string
	recommendedAdjustment       float64
	recommendedAdjustmentString string
	standard                    string
}

// This method takes an absolute file path and passes it to ffmpeg,
// which inspects the loudness of the files and returns its Integrated
// Loudness Summary. This is then parsed via regex and returned as a string
func getFfmpegReadings(filepath string) (string, error) {
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", err
	}
	cmd := exec.Command(binary, "-i", filepath, "-filter_complex", "ebur128", "-f", "null", "-")
	stdout, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	out, err2 := ioutil.ReadAll(stdout)
	if err2 != nil {
		return "", err2
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return string(out), nil
}

// takes an absolute file path, gets the Loudness readings
// returns:
//		passed bool, loudness string, adjustment float64, err error
func checkFileLoudness(filepath string, std bool) (bool, string, float64, error) {
	ffmpegResults, err := getFfmpegReadings(filepath)
	if err != nil {
		log.Fatalln(err)
	}
	// fmt.Println(ffmpegResults)
	ffmpegResults = strings.ToLower(ffmpegResults)

	regexpr := regexp.MustCompile("(?i)i:\\s*(-\\d+.\\d|\\d+.\\d)\\sLUFS")
	result := regexpr.FindAllStringSubmatch(ffmpegResults, -1)

	if len(result) > 0 && len(result[0]) > 0 {
		integratedLoudness := result[len(result)-1][1]
		integratedLoudnessFloat, err := strconv.ParseFloat(integratedLoudness, 64)

		if err != nil {
			return false, "", 0.0, nil
		}

		if std {
			if integratedLoudnessFloat >= -24 && integratedLoudnessFloat <= -22 {
				return true, integratedLoudness, 0.0, nil
			} else {
				return false, integratedLoudness, (-23 - integratedLoudnessFloat), nil
			}
		} else {
			if integratedLoudnessFloat >= -26 && integratedLoudnessFloat <= -22 {
				return true, integratedLoudness, 0.0, nil
			} else {
				return false, integratedLoudness, (-24 - integratedLoudnessFloat), nil
			}
		}
	}
	return false, "", 0.0, errors.New("REGEX PARSE ERROR")

}

// EBU R128 standard = -23 LUFS +/- 1, True Peak -2dB maximum
// ATSC A/85 RP  = -23 LKFS +/- 2, True Peak -2dB maximum

var (
	checkFile    = flag.String("filename", "", "Full path of file to check")
	loudnessStd  = flag.Bool("ebu", true, "True for EBUR 128, False for ATSC A/85 RP")
	autoFix      = flag.Bool("autofix", false, "True to automatically correct the audio levels")
	outputFormat = flag.String("output", "json", "choose: json | xml | simple")
)

func main() {
	flag.Parse()

	err := fcheck(*checkFile)
	if err != nil {
		log.Fatalf("%v\n", err.Error())
	}

	sourceSettings, err := getDataFromFfprobe(*checkFile)
	if err != nil {
		log.Fatalln(err)
	}

	m := new(media)
	json.Unmarshal([]byte(sourceSettings), &m)

	var (
		acodec          string
		audioBitRate    string
		audioChannels   float64
		audioSampleRate string
	)

	for _, stream := range m.streams {
		if stream.codecType == "audio" {
			// log.Printf("AUDIO CODEC: %+v\n", stream.Codec_name)
			acodec = stream.codecName

			// log.Printf("AUDIO BITRATE: %+v\n", stream.Bit_rate)
			audioBitRate = stream.bitRate

			// log.Printf("AUDIO SAMPLE RATE: %+v\n", stream.Sample_rate)
			audioSampleRate = stream.sampleRate

			// log.Printf("AUDIO Channels: %+v\n", stream.Channels)
			audioChannels = stream.channels
		}
	}

	passed, loudness, adjustment, err := checkFileLoudness(*checkFile, *loudnessStd)
	if err != nil {
		log.Fatalln(err)
	}

	var std string

	if *loudnessStd {
		std = "EBU R128 standard = -23 LUFS +/- 1, True Peak -2dB maximum"
	} else {
		std = "ATSC A/85 RP  = -24 LKFS +/- 2, True Peak -2dB maximum"
	}

	mf := mediaFileLoudness{}
	mf.fileName = path.Base(*checkFile)
	mf.passedOrFailed = passed
	mf.loudness = loudness
	mf.recommendedAdjustment = adjustment
	mf.recommendedAdjustmentString = fmt.Sprintf("%.1fdB", adjustment)
	mf.standard = std

	fname := path.Base(*checkFile)
	fdir := path.Dir(*checkFile)
	fEtx := path.Ext(*checkFile)

	outfile := path.Join(fdir, fmt.Sprintf("%s-fixedAudio%s", strings.Split(fname, ".")[0], fEtx))

	switch strings.ToLower(*outputFormat) {
	case "json":
		// create json object
		jsonObj, err := json.MarshalIndent(mf, "", " ")
		if err != nil {
			log.Fatalln(err)
		}
		os.Stdout.Write(jsonObj)
		fmt.Println("")
	case "xml":
		// create xml object
		xmlObj, err := xml.MarshalIndent(mf, "", " ")
		if err != nil {
			log.Fatalln(err)
		}
		os.Stdout.Write(xmlObj)
		fmt.Println("")
	case "simple":
		fmt.Fprintf(os.Stdout, "%s\nLoudness: %s\nAdjustment: %s\nPassed=%t\n", mf.fileName, mf.loudness, mf.recommendedAdjustmentString, mf.passedOrFailed)
	default:
		fmt.Println("File checked to loudness standard: ", std)
		if passed {
			fmt.Printf("FILE IS COMPLIANT TO STANDARD\n")
			fmt.Printf("%s\n", std)
			fmt.Printf("LOUDNESS: %s LUFS\n", loudness)
		} else {
			fmt.Printf("FILE IS NOT COMPLIANT TO STANDARD\n")
			fmt.Printf("%s \n", std)
			fmt.Printf("LOUDNESS: %s LUFS\n", loudness)
			fmt.Printf("RECOMMENDED ADJUSTMENT: %.1fdB\n", adjustment)
			fmt.Printf("IN ORDER TO ACHIEVE THE MEDIAN VALUE\n")
		}
	}

	if !passed {
		if *autoFix {
			ffmpegPath, err := exec.LookPath("ffmpeg")
			if err != nil {
				log.Fatalf("ffmpeg Error: %v", err.Error())
			}
			runCmd := exec.Command(
				ffmpegPath,
				"-threads",
				"auto",
				"-i",
				fmt.Sprintf("%s", *checkFile),
				"-vcodec",
				"copy",
				"-acodec",
				fmt.Sprintf("%s", acodec),
				"-b:a",
				fmt.Sprintf("%v", audioBitRate),
				"-ac",
				fmt.Sprintf("%v", audioChannels),
				"-ar",
				fmt.Sprintf("%v", audioSampleRate),
				"-strict",
				"experimental",
				"-q:v",
				"1",
				"-q:a",
				"1",
				"-filter_complex",
				fmt.Sprintf("volume=volume=%s", mf.recommendedAdjustmentString),
				"-y",
				fmt.Sprintf("%s", outfile),
			)

			err = runCmd.Start()
			if err != nil {
				log.Fatalln("RUN ERROR: ", err.Error())
			}
			err = runCmd.Wait()
		}
	}
}
