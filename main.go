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
	Streams []struct {
		CodecName     string  `json:"codec_name"`
		BitRate       string  `json:"bit_rate"`
		DurationTs    float64 `json:"duration_ts"`
		CodecType     string  `json:"codec_type"`
		CodecTimeBase string  `json:"codec_time_base"`
		Profile       string  `json:"Profile"`
		Tags          struct {
			Language         string `json:"language"`
			MajorBrand       string `json:"major_brand"`
			MinorVersion     string `json:"minor_version"`
			CompatibleBrands string `json:"compatible_brands"`
			HandlerName      string `json:"handler_name"`
		}
		Duration           string  `json:"duration"`
		Index              float64 `json:"index"`
		SampleAspectRatio  string  `json:"sample_aspect_ratio"`
		CodecTag           string  `json:"codec_tag"`
		CodecTagString     string  `json:"codec_tag_string"`
		StartPts           float64 `json:"start_pts"`
		SampleRate         string  `json:"sample_rate"`
		NbFrames           string  `json:"nb_frames"`
		AvgFrameRate       string  `json:"avg_frame_rate"`
		CodecLongName      string  `json:"codec_long_name"`
		DisplayAspectRatio string  `json:"display_aspect_ratio"`
		Level              float64 `json:"level"`
		HasBFrames         float64 `json:"has_b_frames"`
		PixFmt             string  `json:"pix_fmt"`
		TimeBase           string  `json:"time_base"`
		Height             float64 `json:"height"`
		BitsPerSample      float64 `json:"bits_per_sample"`
		Width              float64 `json:"width"`
		SampleFmt          string  `json:"sample_fmt"`
		StartTime          string  `json:"start_time"`
		Channels           float64 `json:"channels"`
		RFrameRate         string  `json:"r_frame_rate"`
	}
	Format struct {
		FormatName     string `json:"format_name"`
		FormatLongName string `json:"ormat_long_name"`
		NbStreams      int    `json:"nb_streams"`
		Duration       string `json:"duration"`
		StartTime      string `json:"start_time"`
		Size           string `json:"size"`
		Filename       string `json:"filename"`
		BitRate        string `json:"filename"`
		Tags           struct {
			Language         string `json:"language"`
			MajorBrand       string `json:"major_brand"`
			MinorVersion     string `json:"minor_version"`
			CompatibleBrands string `json:"compatible_brands"`
			HandlerName      string `json:"handler_name"`
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
	FileName                    string
	PassedOrFailed              bool
	Loudness                    string
	RecommendedAdjustment       float64
	RecommendedAdjustmentString string
	Standard                    string
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
		Acodec          string
		AudioBitRate    string
		AudioChannels   float64
		AudioSampleRate string
	)

	for _, stream := range m.Streams {
		if stream.CodecType == "audio" {
			// log.Printf("AUDIO CODEC: %+v\n", stream.Codec_name)
			Acodec = stream.CodecName

			// log.Printf("AUDIO BITRATE: %+v\n", stream.Bit_rate)
			AudioBitRate = stream.BitRate

			// log.Printf("AUDIO SAMPLE RATE: %+v\n", stream.Sample_rate)
			AudioSampleRate = stream.SampleRate

			// log.Printf("AUDIO Channels: %+v\n", stream.Channels)
			AudioChannels = stream.Channels
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
	mf.FileName = path.Base(*checkFile)
	mf.PassedOrFailed = passed
	mf.Loudness = loudness
	mf.RecommendedAdjustment = adjustment
	mf.RecommendedAdjustmentString = fmt.Sprintf("%.1fdB", adjustment)
	mf.Standard = std

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
		fmt.Fprintf(os.Stdout, "%s\nLoudness: %s\nAdjustment: %s\nPassed=%t\n", mf.FileName, mf.Loudness, mf.RecommendedAdjustmentString, mf.PassedOrFailed)
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
				fmt.Sprintf("%s", Acodec),
				"-b:a",
				fmt.Sprintf("%v", AudioBitRate),
				"-ac",
				fmt.Sprintf("%v", AudioChannels),
				"-ar",
				fmt.Sprintf("%v", AudioSampleRate),
				"-strict",
				"experimental",
				"-q:v",
				"1",
				"-q:a",
				"1",
				"-filter_complex",
				fmt.Sprintf("volume=volume=%s", mf.RecommendedAdjustmentString),
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
