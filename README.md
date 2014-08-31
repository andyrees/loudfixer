loudfixer
=========

A simple utilty to check the audio levels in a media file for compliance against the EBU R128 or ATSC A/85 RP standards

This utility uses the power of Go to easily parse the json results from a ffprobe call on a file and obtains the specifics of each stream in the media file. It then uses ffmpeg to check the file for its EBU loudness levels and returns the value.

Once this is done it is possible to create a ffmpeg command to transcode the file according to the source settings, thus avoiding audio drift and any obvious deterioration, whilst correcting the audio levels and ensuring broadcast loudness audio level compliance.

Andy Rees
