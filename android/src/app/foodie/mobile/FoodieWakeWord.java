package app.foodie.mobile;

import android.content.Context;
import com.k2fsa.sherpa.onnx.*;
import java.io.*;

/** Small streaming keyword model. Microphone samples never leave this class in wake mode. */
final class FoodieWakeWord implements AutoCloseable {
 private KeywordSpotter spotter;
 private OnlineStream stream;
 private int samples;
 FoodieWakeWord(Context context) throws IOException {
  File directory=new File(context.getNoBackupFilesDir(),"foodie-kws-1.13.8");
  if(!directory.isDirectory()&&!directory.mkdirs())throw new IOException("Kunne ikke klargøre Foodie.");
  for(String name:new String[]{"encoder.onnx","decoder.onnx","joiner.onnx","tokens.txt","keywords.txt"}) {
   File file=new File(directory,name);
   // APK updates may tune the wake phrase; the signed asset remains authoritative.
   if(!file.exists()||name.endsWith(".txt")) {
    File temporary=new File(directory,name+".tmp");
    try(InputStream in=context.getAssets().open("foodie/"+name);FileOutputStream out=new FileOutputStream(temporary)){byte[] buffer=new byte[32768];int n;while((n=in.read(buffer))!=-1)out.write(buffer,0,n);out.getFD().sync();}
    if(!temporary.renameTo(file))throw new IOException("Kunne ikke gemme talemodellen.");
   }
  }
  OnlineTransducerModelConfig transducer=OnlineTransducerModelConfig.builder().setEncoder(new File(directory,"encoder.onnx").getPath()).setDecoder(new File(directory,"decoder.onnx").getPath()).setJoiner(new File(directory,"joiner.onnx").getPath()).build();
  OnlineModelConfig model=OnlineModelConfig.builder().setTransducer(transducer).setTokens(new File(directory,"tokens.txt").getPath()).setNumThreads(1).setDebug(false).setProvider("cpu").build();
  KeywordSpotterConfig config=KeywordSpotterConfig.builder().setOnlineModelConfig(model).setKeywordsFile(new File(directory,"keywords.txt").getPath()).setKeywordsScore(1.8f).setKeywordsThreshold(.32f).setNumTrailingBlanks(2).build();
  spotter=new KeywordSpotter(config);reset();
 }
 void reset(){if(stream!=null)stream.release();stream=spotter.createStream();samples=0;}
 boolean accept(float[] frames){stream.acceptWaveform(frames,16000);samples+=frames.length;while(spotter.isReady(stream)){spotter.decode(stream);if(!spotter.getResult(stream).getKeyword().isEmpty()){reset();return true;}}if(samples>16000*30)reset();return false;}
 @Override public void close(){if(stream!=null){stream.release();stream=null;}if(spotter!=null){spotter.release();spotter=null;}}
}
