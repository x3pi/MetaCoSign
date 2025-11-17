
# cd 3rdparty
# bash build.sh
# cd ..

cd c_mvm
rm -rf build
mkdir build 
cd build
cmake ../ 
make install

cd ../../linker
rm -rf build
mkdir build 
cd build
cmake ..
make install
 
