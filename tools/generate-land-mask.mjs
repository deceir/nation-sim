import fs from 'node:fs';
import {createRequire} from 'node:module';
const sharp=createRequire(import.meta.url)('sharp');

const width=1440,height=720;
const svg=Buffer.from(fs.readFileSync(new URL('../public/world-map.svg',import.meta.url),'utf8').replace(/<path id="ocean"[\s\S]*?\/>/,''));
const {data,info}=await sharp(svg).resize(width,height,{fit:'fill'}).ensureAlpha().raw().toBuffer({resolveWithObject:true});
const bits=Buffer.alloc(Math.ceil(width*height/8));
for(let pixel=0;pixel<width*height;pixel++)if(data[pixel*info.channels+3]>=32)bits[pixel>>3]|=1<<(pixel&7);
const encoded=bits.toString('base64');
fs.writeFileSync(new URL('../src/landMask.ts',import.meta.url),`// Generated from public/world-map.svg by tools/generate-land-mask.mjs.\nconst width=${width},height=${height},encoded='${encoded}';\nlet decoded:Uint8Array|undefined;\nfunction bytes(){if(decoded)return decoded;const raw=atob(encoded);decoded=Uint8Array.from(raw,c=>c.charCodeAt(0));return decoded}\nexport function isLand(latitude:number,longitude:number){if(!Number.isFinite(latitude)||!Number.isFinite(longitude)||latitude < -90||latitude > 90||longitude < -180||longitude > 180)return false;const x=Math.min(width-1,Math.max(0,Math.floor((longitude+180)/360*width))),y=Math.min(height-1,Math.max(0,Math.floor((90-latitude)/180*height))),index=y*width+x,data=bytes();return (data[index>>3]&(1<<(index&7)))!==0}\n`);
fs.writeFileSync(new URL('../cmd/server/land_mask_generated.go',import.meta.url),`// Code generated from public/world-map.svg by tools/generate-land-mask.mjs; DO NOT EDIT.\npackage main\n\nimport \"encoding/base64\"\n\nconst landMaskWidth=${width}\nconst landMaskHeight=${height}\nconst encodedLandMask=\"${encoded}\"\n\nvar landMask=func()[]byte{value,err:=base64.StdEncoding.DecodeString(encodedLandMask);if err!=nil{panic(err)};return value}()\n\nfunc isLandPoint(lat,lng float64)bool{\n if lat < -90||lat > 90||lng < -180||lng > 180{return false}\n x:=int((lng+180)/360*landMaskWidth); y:=int((90-lat)/180*landMaskHeight)\n if x<0{x=0};if x>=landMaskWidth{x=landMaskWidth-1};if y<0{y=0};if y>=landMaskHeight{y=landMaskHeight-1}\n index:=y*landMaskWidth+x\n return landMask[index>>3]&(1<<uint(index&7))!=0\n}\n`);
console.log(`Generated ${width}x${height} land mask (${bits.length} bytes).`);
