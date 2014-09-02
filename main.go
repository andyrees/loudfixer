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

type Media struct {
	Streams []struct {
		Codec_name      string  `json:"codec_name"`
		Bit_rate        string  `json:"bit_rate"`
		Duration_ts     float64 `json:"duration_ts"`
		Codec_type      string  `json:"codec_type"`
		Codec_time_base string  `json:"codec_time_base"`
		Profile         string  `json:"Profile"`
		Tags            struct {
			Language          string `json:"language"`
			Major_brand       string `json:"major_brand"`
			Minor_version     string `json:"minor_version"`
			Compatible_brands string `json:"compatible_brands"`
			Handler_name      string `json:"handler_name"`
		}
		Duration             string  `json:"duration"`
		Index                float64 `json:"index"`
		Sample_aspect_ratio  string  `json:"sample_aspect_ratio"`
		Codec_tag            string  `json:"codec_tag"`
		Codec_tag_string     string  `json:"codec_tag_string"`
		Start_pts            float64 `json:"start_pts"`
		Sample_rate          string  `json:"sample_rate"`
		Nb_frames            string  `json:"nb_frames"`
		Avg_frame_rate       string  `json:"avg_frame_rate"`
		Codec_long_name      string  `json:"codec_long_name"`
		Display_aspect_ratio string  `json:"display_aspect_ratio"`
		Level                float64 `json:"level"`
		Has_b_frames         float64 `json:"has_b_frames"`
		Pix_fmt              string  `json:"pix_fmt"`
		Time_base            string  `json:"time_base"`
		Height               float64 `json:"height"`
		Bits_per_sample      float64 `json:"bits_per_sample"`
		Width                float64 `json:"width"`
		Sample_fmt           string  `json:"sample_fmt"`
		Start_time           string  `json:"start_time"`
		Channels             float64 `json:"channels"`
		R_frame_rate         string  `json:"r_frame_rate"`
	}
	Format struct {
		Format_name      string `json:"format_name"`
		Format_long_name string `json:"ormat_long_name"`
		Nb_streams       int    `json:"nb_streams"`
		Duration         string `json:"duration"`
		Start_time       string `json:"start_time"`
		Size             string `json:"size"`
		Filename         string `json:"filename"`
		Bit_rate         string `json:"filename"`
		Tags             struct {
			Language          string `json:"language"`
			Major_brand       string `json:"major_brand"`
			Minor_version     string `json:"minor_version"`
			Compatible_brands string `json:"compatible_brands"`
			Handler_name      string `json:"handler_name"`
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

func get_data_from_ffprobe(filepath string) (string, error) {
	ffprobe_path, lookerr := exec.LookPath("ffprobe")
	if lookerr != nil {
		return "", lookerr
	}

	json_result_bytes, err := exec.Command(ffprobe_path, "-show_format", "-show_streams", "-print_format", "json=c=1", filepath).Output()
	if err != nil {
		return "", err
	}

	return string(json_result_bytes), nil
}

type MediaFileLoudness struct {
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
func check_file_loudness(filepath string, std bool) (bool, string, float64, error) {
	ffmpegResults, err := getFfmpegReadings(filepath)
	if err != nil {
		log.Fatalln(err)
	}
	// fmt.Println(ffmpegResults)
	ffmpegResults = strings.ToLower(ffmpegResults)

	regexpr := regexp.MustCompile("(?i)i:\\s*(-\\d+.\\d|\\d+.\\d)\\sLUFS")
	result := regexpr.FindAllStringSubmatch(ffmpegResults, -1)

	if len(result) > 0 && len(result[0]) > 0 {
		integrated_loudness := result[len(result)-1][1]
		integrated_loudness_float, err := strconv.ParseFloat(integrated_loudness, 64)

		if err != nil {
			return false, "", 0.0, nil
		}

		if std {
			if integrated_loudness_float >= -24 && integrated_loudness_float <= -22 {
				return true, integrated_loudness, 0.0, nil
			} else {
				return false, integrated_loudness, (-23 - integrated_loudness_float), nil
			}
		} else {
			if integrated_loudness_float >= -26 && integrated_loudness_float <= -22 {
				return true, integrated_loudness, 0.0, nil
			} else {
				return false, integrated_loudness, (-24 - integrated_loudness_float), nil
			}
		}
		return false, integrated_loudness, 0.0, nil
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

	source_settings, err := get_data_from_ffprobe(*checkFile)
	if err != nil {
		log.Fatalln(err)
	}

	m := new(Media)
	json.Unmarshal([]byte(source_settings), &m)

	var (
		acodec          string
		AudioBitRate    string
		AudioChannels   float64
		AudioSampleRate string
	)

	for _, stream := range m.Streams {
		if stream.Codec_type == "audio" {
			// log.Printf("AUDIO CODEC: %+v\n", stream.Codec_name)
			acodec = stream.Codec_name

			// log.Printf("AUDIO BITRATE: %+v\n", stream.Bit_rate)
			AudioBitRate = stream.Bit_rate

			// log.Printf("AUDIO SAMPLE RATE: %+v\n", stream.Sample_rate)
			AudioSampleRate = stream.Sample_rate

			// log.Printf("AUDIO Channels: %+v\n", stream.Channels)
			AudioChannels = stream.Channels
		}
	}

	passed, loudness, adjustment, err := check_file_loudness(*checkFile, *loudnessStd)
	if err != nil {
		log.Fatalln(err)
	}

	var std string

	if *loudnessStd {
		std = "EBU R128 standard = -23 LUFS +/- 1, True Peak -2dB maximum"
	} else {
		std = "ATSC A/85 RP  = -24 LKFS +/- 2, True Peak -2dB maximum"
	}

	mf := MediaFileLoudness{}
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
		json_obj, err := json.MarshalIndent(mf, "", " ")
		if err != nil {
			log.Fatalln(err)
		}
		os.Stdout.Write(json_obj)
		fmt.Println("")
	case "xml":
		// create xml object
		xml_obj, err := xml.MarshalIndent(mf, "", " ")
		if err != nil {
			log.Fatalln(err)
		}
		os.Stdout.Write(xml_obj)
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
			ffmpeg_path, err := exec.LookPath("ffmpeg")
			if err != nil {
				log.Fatalf("ffmpeg Error: %v", err.Error())
			}
			runCmd := exec.Command(
				ffmpeg_path,
				"-threads",
				"auto",
				"-i",
				fmt.Sprintf("%s", *checkFile),
				"-vcodec",
				"copy",
				"-acodec",
				fmt.Sprintf("%s", acodec),
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
