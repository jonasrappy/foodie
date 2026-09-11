/*!
 * Bundled Three.js license:
 * The MIT License
 * 
 * Copyright © 2010-2026 three.js authors
 * 
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 * 
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */
import * as THREE from 'three';

// A small procedural model: no remote assets, tracking, camera or microphone access.
export function createFoodie(mount, reducedMotion) {
  const renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true, powerPreference: 'low-power' });
  renderer.setPixelRatio(Math.min(devicePixelRatio || 1, 1.5));
  renderer.setClearColor(0x000000, 0);
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.12;
  const canvas = renderer.domElement;
  canvas.className = 'foodie-canvas'; canvas.setAttribute('aria-hidden', 'true');
  mount.append(canvas);
  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(31, 1, .1, 40);
  camera.position.set(0, 1.22, 7.6); camera.lookAt(0, .43, 0);
  scene.add(new THREE.HemisphereLight(0xfff5df, 0x819b80, 2.4));
  function light(color, intensity, x, y, z) {
    const lamp = new THREE.DirectionalLight(color, intensity); lamp.position.set(x, y, z); scene.add(lamp);
  }
  light(0xfff4dc, 3.5, -3, 5, 4); light(0xd9eadc, 1.1, 4, 2, 3); light(0xffe0aa, 2.4, 1, 4, -3);
  const root = new THREE.Group(); scene.add(root);
  const material = (color, roughness = .38) => new THREE.MeshStandardMaterial({ color, roughness, metalness: 0 });
  const cream = material(0xf8efd8, .3), eyeMaterial = material(0x193328, .2);
  const blush = material(0xeaa797, .5), gold = material(0xe7ae69, .38), vein = material(0xba8148, .55);
  const white = new THREE.MeshBasicMaterial({ color: 0xfffdf0 });
  const mouthMaterial = material(0x1e3228, .7);
  const sphereGeometry = new THREE.SphereGeometry(1, 24, 16);
  function ellipsoid(parent, mat, x, y, z, sx, sy, sz) {
    const mesh = new THREE.Mesh(sphereGeometry, mat); mesh.position.set(x, y, z); mesh.scale.set(sx, sy, sz); parent.add(mesh); return mesh;
  }
  function tube(parent, points, radius, mat, segments = 28) {
    const curve = new THREE.CatmullRomCurve3(points.map(p => new THREE.Vector3(...p)));
    const mesh = new THREE.Mesh(new THREE.TubeGeometry(curve, segments, radius, 8, false), mat); parent.add(mesh); return mesh;
  }
  const profile = [[0,-.86],[.32,-.86],[.67,-.74],[.96,-.53],[1.2,-.19],[1.36,.25],[1.42,.69],[1.4,.79],[1.33,.79],[1.29,.65],[1.24,.27],[1.06,-.18],[.8,-.48],[.4,-.68],[0,-.69]];
  const profileCurve = new THREE.CatmullRomCurve3(profile.map(([x,y]) => new THREE.Vector3(x,y,0)));
  const points = profileCurve.getPoints(72).map(p => new THREE.Vector2(Math.max(0,p.x),p.y));
  const bowl = new THREE.Mesh(new THREE.LatheGeometry(points, 64), cream); bowl.scale.z = .78; root.add(bowl);
  const rim = new THREE.Mesh(new THREE.TorusGeometry(1.365,.055,10,64),cream);
  rim.rotation.x = Math.PI/2; rim.scale.y = .78; rim.position.y = .775; root.add(rim);
  ellipsoid(root, cream, -.54,-.84,.25,.27,.14,.34); ellipsoid(root, cream, .54,-.84,.25,.27,.14,.34);
  const arms = [-1,1].map(side => {
    const arm = new THREE.Group(); arm.position.set(side*1.27,-.06,0); root.add(arm);
    ellipsoid(arm,cream,side*.16,-.015,.03,.24,.11,.12);
    ellipsoid(arm,cream,side*.35,.025,.05,.16,.15,.145); return arm;
  });
  const eyes = [-1,1].map(side => {
    const eye = new THREE.Group(); eye.position.set(side*.53,.03,.977); eye.rotation.y=side*.33; root.add(eye);
    ellipsoid(eye,eyeMaterial,0,0,0,.18,.245,.10);
    ellipsoid(eye,white,-.048,.085,.088,.049,.071,.018);
    ellipsoid(eye,white,.053,-.073,.092,.023,.029,.01);
    return eye;
  });
  [-1,1].forEach(side => {
    const cheek=ellipsoid(root,blush,side*.91,-.22,.733,.20,.095,.024); cheek.rotation.y=side*.67; cheek.rotation.z=side*.08;
  });
  const smile=tube(root,[[-.21,-.265,.987],[-.12,-.355,1.00],[0,-.385,1.01],[.12,-.355,1.00],[.21,-.265,.987]],.026,mouthMaterial);
  const mouth = new THREE.Group(); mouth.position.set(0,-.255,.987); root.add(mouth);
  const mouthShape = new THREE.Shape(); mouthShape.moveTo(-.23,0); mouthShape.quadraticCurveTo(0,-.045,.23,0);
  mouthShape.bezierCurveTo(.22,-.29,-.22,-.29,-.23,0);
  const mouthMesh = new THREE.Mesh(new THREE.ExtrudeGeometry(mouthShape,{depth:.012,bevelEnabled:true,bevelSegments:2,steps:1,bevelSize:.008,bevelThickness:.008,curveSegments:18}),mouthMaterial);
  mouth.add(mouthMesh); ellipsoid(mouth,blush,0,-.168,.026,.115,.055,.013);
  const leaf = new THREE.Group(); leaf.position.set(.03,.82,-.03); root.add(leaf);
  tube(leaf,[[0,0,0],[.07,.25,0],[.16,.37,.03]],.027,vein);
  // An indexed curved surface gives the leaf continuous shading as it turns.
  const leafVertices=[],leafIndices=[],rows=28,columns=10;
  for(let row=0;row<=rows;row++)for(let column=0;column<=columns;column++){
    const t=row/rows,u=column/columns*2-1,width=Math.sin(t*Math.PI)*.32;
    leafVertices.push(.06+.77*t-width*u*.8,.23+1.03*t+width*u*.6,.07+.16*t+.11*(1-u*u)*Math.sin(t*Math.PI));
  }
  for(let row=0;row<rows;row++)for(let column=0;column<columns;column++){
    const a=row*(columns+1)+column,b=a+columns+1;leafIndices.push(a,a+1,b,a+1,b+1,b);
  }
  const leafGeometry=new THREE.BufferGeometry();leafGeometry.setAttribute('position',new THREE.Float32BufferAttribute(leafVertices,3));leafGeometry.setIndex(leafIndices);leafGeometry.computeVertexNormals();
  gold.side=THREE.DoubleSide;leaf.add(new THREE.Mesh(leafGeometry,gold));
  const veinPoints=[];for(let i=0;i<=16;i++){const t=i/16;veinPoints.push([.06+.77*t,.23+1.03*t,.087+.16*t+.11*Math.sin(t*Math.PI)]);}
  tube(leaf,veinPoints,.013,vein);
  const steamMaterial = new THREE.MeshStandardMaterial({color:0xfaf5df,transparent:true,opacity:.45,roughness:.8,depthWrite:false});
  const steam = tube(root,[[-.63,.98,0],[-.74,1.19,0],[-.57,1.41,.01],[-.61,1.59,.02]],.036,steamMaterial);
  // A tiny radial contact shadow avoids an expensive real-time shadow pass.
  const shadowPixels = new Uint8Array(64*64*4);
  for(let y=0;y<64;y++)for(let x=0;x<64;x++){
    const i=(y*64+x)*4, r=Math.hypot((x-31.5)/31.5,(y-31.5)/31.5);
    shadowPixels[i]=13;shadowPixels[i+1]=34;shadowPixels[i+2]=24;shadowPixels[i+3]=Math.round(Math.pow(Math.max(0,1-r),2)*95);
  }
  const shadowTexture = new THREE.DataTexture(shadowPixels,64,64); shadowTexture.needsUpdate=true;
  const shadow = new THREE.Mesh(new THREE.PlaneGeometry(3.5,2.3),new THREE.MeshBasicMaterial({map:shadowTexture,transparent:true,depthWrite:false}));
  shadow.rotation.x=-Math.PI/2;shadow.position.set(0,-1.025,.15);scene.add(shadow);

  let state={visible:false,phase:'waking',level:0}, elapsed=0,lastTime=0,level=0,aperture=.2;
  let nextBlink=2.7,blinkStarted=-10,blink=1,live=false,lost=false,disposed=false,ready=false;
  let lastPhase='',listenBlend=0,talkBlend=0,entry=.86;
  const damp=(a,b,rate,dt)=>a+(b-a)*(1-Math.exp(-rate*dt));
  function frame(now) {
    if(disposed||lost)return;
    const calm=reducedMotion.matches;
    const dt=calm?0:Math.min(.05,lastTime?Math.max(0,(now-lastTime)/1000):1/60);lastTime=now;elapsed+=dt;
    const t=elapsed;
    const listening=state.phase==='listening'||state.phase==='hearing',speaking=state.phase==='speaking';
    listenBlend=calm?(listening?1:0):damp(listenBlend,listening?1:0,6,dt);
    talkBlend=calm?(speaking?1:0):damp(talkBlend,speaking?1:0,9,dt);
    level=calm?0:damp(level,listening?state.level:0,10,dt);
    entry=calm?1:damp(entry,1,7,dt);
    const breath=calm?0:Math.sin(t*1.8);
    root.position.y=calm?0:breath*.027+Math.sin(t*.71)*.012;
    root.rotation.y=calm?0:Math.sin(t*.67)*.13+Math.sin(t*1.17)*.025;
    root.rotation.x=calm?0:Math.sin(t*.83)*.025-listenBlend*.035+talkBlend*Math.sin(t*3.4)*.018;
    root.rotation.z=calm?-listenBlend*.055:Math.sin(t*.93)*.022-listenBlend*.065-level*.014;
    root.scale.set(entry*(1-breath*.0025),entry*(1+breath*.005),entry);
    leaf.rotation.z=calm?0:Math.sin(t*1.65-.7)*.08+Math.sin(t*.57)*.035;
    leaf.rotation.x=calm?0:Math.sin(t*1.12)*.09;
    arms[0].rotation.z=.35+(calm?0:Math.sin(t*1.65+.4)*.07-talkBlend*.06);
    arms[1].rotation.z=-.35+(calm?0:Math.sin(t*1.65-1)*.08)+listenBlend*.73;
    arms[1].rotation.x=listenBlend*-.22;
    steam.position.y=calm?0:Math.sin(t*1.6)*.045;steamMaterial.opacity=calm?.45:.35+.10*Math.sin(t*1.6);
    if(!calm&&t>=nextBlink){blinkStarted=t;nextBlink=t+3.8+Math.random()*3;}
    const blinkProgress=(t-blinkStarted)/.18;
    blink=calm?1:blinkProgress>=0&&blinkProgress<1?1-.94*Math.pow(Math.sin(blinkProgress*Math.PI),2):1;
    eyes.forEach((eye,i)=>{
      eye.scale.y=blink*(1+listenBlend*.045);
      eye.position.y=.03+(calm?0:Math.sin(t*.79)*.009)+listenBlend*.012;
      eye.position.x=(i?1:-1)*.53+(calm?0:Math.sin(t*.53)*.012);
    });
    // The mouth closes immediately when native playback ends; soft variation while it speaks.
    mouth.visible=speaking;smile.visible=!speaking;
    const syllable=.25+.63*Math.abs(Math.sin(t*13.1)*.7+Math.sin(t*19.7)*.3);
    aperture=calm?.8:damp(aperture,syllable,24,dt);mouth.scale.y=aperture;mouth.scale.x=1-(1-aperture)*.07;
    shadow.scale.setScalar(1-(calm?0:(breath+1)*.015));
    renderer.render(scene,camera);
    if(!ready){ready=true;mount.classList.add('has-3d');}
  }
  function sync() {
    const shouldRun=state.visible&&!document.hidden&&!lost&&!disposed;
    if(!shouldRun){if(live){renderer.setAnimationLoop(null);live=false;}lastTime=0;return;}
    if(reducedMotion.matches){renderer.setAnimationLoop(null);live=false;frame(performance.now());}
    else if(!live){lastTime=0;live=true;renderer.setAnimationLoop(frame);}
  }
  function resize(){const width=mount.clientWidth||360,height=mount.clientHeight||360;renderer.setSize(width,height,false);camera.aspect=width/height;camera.updateProjectionMatrix();if(state.visible&&!live)frame(performance.now());}
  const observer=new ResizeObserver(resize);observer.observe(mount);resize();
  const onLost=event=>{event.preventDefault();lost=true;ready=false;mount.classList.remove('has-3d');sync();};
  const onRestored=()=>{lost=false;sync();};
  canvas.addEventListener('webglcontextlost',onLost);canvas.addEventListener('webglcontextrestored',onRestored);
  document.addEventListener('visibilitychange',sync);reducedMotion.addEventListener('change',sync);
  return {
    update(next) {
      const wasVisible=state.visible;
      const changed=lastPhase!==next.phase||wasVisible!==next.visible;
      state={visible:!!next.visible,phase:next.phase,level:Math.max(0,Math.min(1,Number(next.level)||0))};
      lastPhase=state.phase;
      if(state.visible&&!wasVisible){entry=.86;lastTime=0;resize();}
      // RMS does not request frames when motion has explicitly been reduced.
      if(changed||!reducedMotion.matches)sync();
    },
    dispose() {
      disposed=true;renderer.setAnimationLoop(null);observer.disconnect();
      document.removeEventListener('visibilitychange',sync);reducedMotion.removeEventListener('change',sync);
      canvas.removeEventListener('webglcontextlost',onLost);canvas.removeEventListener('webglcontextrestored',onRestored);
      const geometries=new Set(),materials=new Set();
      scene.traverse(object=>{if(object.geometry)geometries.add(object.geometry);if(object.material)materials.add(object.material);});
      geometries.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());shadowTexture.dispose();renderer.dispose();renderer.forceContextLoss();canvas.remove();mount.classList.remove('has-3d');
    }
  };
}
