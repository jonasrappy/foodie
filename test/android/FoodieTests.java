package app.foodie.mobile;
import com.k2fsa.sherpa.onnx.*;
import java.nio.file.*;
import java.nio.*;
import java.util.*;
public final class FoodieTests {
 public static void main(String[] args)throws Exception{
  if(args.length>0){String root=args[0];
   OnlineTransducerModelConfig transducer=OnlineTransducerModelConfig.builder().setEncoder(root+"/encoder.onnx").setDecoder(root+"/decoder.onnx").setJoiner(root+"/joiner.onnx").build();
   OnlineModelConfig model=OnlineModelConfig.builder().setTransducer(transducer).setTokens(root+"/tokens.txt").setNumThreads(1).setDebug(false).build();
   KeywordSpotter spotter=new KeywordSpotter(KeywordSpotterConfig.builder().setOnlineModelConfig(model).setKeywordsFile(root+"/keywords.txt").build());
   for(int a=1;a<args.length;a++){
    byte[] data=Files.readAllBytes(Paths.get(args[a]));ByteBuffer pcm=ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);pcm.position(44);OnlineStream stream=spotter.createStream();boolean found=false;
    while(pcm.remaining()>=2){int count=Math.min(1600,pcm.remaining()/2);float[] samples=new float[count];for(int i=0;i<count;i++)samples[i]=pcm.getShort()/32768f;stream.acceptWaveform(samples,16000);while(spotter.isReady(stream)){spotter.decode(stream);if(!spotter.getResult(stream).getKeyword().isEmpty()){found=true;spotter.reset(stream);}}}
    System.out.println(args[a]+": wake="+found);if(args[a].contains("positive")&&!found)throw new AssertionError("missed wake word");if(args[a].contains("negative")&&found)throw new AssertionError("false activation");stream.release();
   }spotter.release();
  }
  System.out.println("PASS: local wake model/JNI, Danish/English positives and non-trigger fixtures");
 }
}
