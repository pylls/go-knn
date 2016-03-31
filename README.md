# go-kNN
This is a a golang implementation of the kNN website fingerprinting attack and feature extractor by
[Wang et al.](https://crysp.uwaterloo.ca/software/webfingerprint/). There are two versions:

1. `knn.orig` which produces identical output as Wang's implementation in the
[attacks collection](https://crysp.uwaterloo.ca/software/webfingerprint/).
2. `knn.fixed` that fixes some bugs in feature extraction, removes unnecessary features for Tor,
and fixes a bug in the weight learning algorithm. The feature extraction is (to the best of my
  knowledge) identical to [Table 3.1 in Wang's PhD thesis](https://uwspace.uwaterloo.ca/bitstream/handle/10012/10123/Wang_Tao.pdf).

Initial tests shows _no meaningful differences_ between the two versions beyond speed due to
removing unnecessary features for Tor.
