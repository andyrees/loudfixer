loudfixer
=========

A simple utilty to check the audio levels in a media file for compliance against the EBU R128 or ATSC A/85 RP standards.

[Audio loudness measurement and normalization with EBU R128 (Calm Act, ATSC A/85](<http://auphonic.com/blog/15/>)<sup><a href="#fn1" id="ref1">1</a></sup>


This utility uses the power of Go to easily parse the json results from a ffprobe call on a file and obtains the specifics of each stream in the media file. It then uses ffmpeg to check the file for its EBU loudness levels and returns the value.

Once this is done it is possible to create a ffmpeg command to transcode the file according to the source settings, thus avoiding audio drift and any obvious deterioration, whilst correcting the audio levels and ensuring broadcast loudness audio level compliance.

Andy Rees


	Usage of loudfixer:
  	-autofix=false: True to automatically correct the audio levels
  	-ebu=true: True for EBUR 128, False for ATSC A/85 RP
  	-filename="": Full path of file to check
  	-output="json": choose: json | xml | simple
  	
<sup id="fn1">1. [GRH. (2012). Audio loudness measurement and normalization with EBU R128 (Calm Act, ATSC A/85). Available: http://auphonic.com/blog/15/. Last accessed 31 Aug 2014.]<a href="#ref1" title="Jump back to footnote 1 in the text.">↩</a></sup>
