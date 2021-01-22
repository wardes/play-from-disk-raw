# Play Raw audio test

This is a copy of the Pion play-from-disk code, modified to play from a raw audiofile of litten-endian float32 stereo 48000hz samples.

Create a file 'output.raw' in the current directory. You can use Audacity for example, open an mp3 and resample to 48000hz, and export as raw with no header, float32.

Open jsfiddle/demo.html in a browser, you can use:

```
python3 -m http.server
```
To serve it over http if you want.


# Issues
The audio stutters from time to time