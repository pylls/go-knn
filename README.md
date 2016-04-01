# go-kNN
This is a a golang implementation of the kNN website fingerprinting attack and feature extractor by
[Wang et al.](https://crysp.uwaterloo.ca/software/webfingerprint/).

There are two versions:

- `knn.orig` which produces identical output as Wang's implementation in the
[attacks collection](https://crysp.uwaterloo.ca/software/webfingerprint/attacks.zip).
- `knn.fixed` that fixes some bugs in feature extraction, removes unnecessary features for Tor,
and fixes a bug in the weight learning algorithm. The feature extraction is (to the best of my
  knowledge) identical to [Table 3.1 in Wang's PhD thesis](https://uwspace.uwaterloo.ca/bitstream/handle/10012/10123/Wang_Tao.pdf).

Initial tests show _no meaningful differences_ between the two versions beyond speed due to
removing unnecessary features for Tor from the feature extraction. The port and bugfixes were a
way for me to better understand the attack.

As far as a straight port from source code can be licensed by me (probably not at all),
the code is licensed under GPLv3.

This work is part of the [HOT research project](http://www.cs.kau.se/pulls/hot/), funded by the
 [Swedish Internet Fund](https://www.internetfonden.se/om/the-internet-fund/).
