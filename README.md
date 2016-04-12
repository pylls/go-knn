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

## Example
Download Wang et al.'s [cell traces](https://crysp.uwaterloo.ca/software/webfingerprint/knndata.zip)
 for their USENIX 2014 paper with 100x90 + 900 traces. Extract the traces, creating the `batch` folder. 

Extract the original features:

    $ go run src/features/knn.orig/fextractor.go -sites 100 -instances 90 -open 9000
    2016/04/12 11:31:15 starting parsing...
    2016/04/12 11:32:27 done parsing (100 sites, 90 instances, 9000 open world, folder "batch/", suffix "f")

Extract the fixed features:

    $ go run src/features/knn.fixed/fextractor.go -sites 100 -instances 90 -open 9000
    2016/04/12 11:30:29 starting parsing...
    2016/04/12 11:30:46 done parsing (100 sites, 90 instances, 9000 open world, folder "batch/", suffix "s")

Run the original attack:

    $ go run src/attacks/knn.orig/knn.orig.go
    2016/04/12 11:32:49 loaded instances: main
    2016/04/12 11:32:50 loaded instances: training
    2016/04/12 11:32:52 loaded instances: testing
    2016/04/12 11:32:55 loaded instances: open
    2016/04/12 11:32:55 starting to learn distance...
    	distance... 8999 (0-9000)
    2016/04/12 11:47:18 finished
    2016/04/12 11:47:18 started computing accuracy...
    	accuracy... 17999 (0-18000)
    2016/04/12 12:43:11 finished
    2016/04/12 12:43:11 Accuracy: 0.870667 0.989889

Run the fixed attack:

    $ go run src/attacks/knn.fixed/knn.fixed.go 
    2016/04/12 11:32:45 loaded instances: main
    2016/04/12 11:32:46 loaded instances: training
    2016/04/12 11:32:46 loaded instances: testing
    2016/04/12 11:32:46 loaded instances: open
    2016/04/12 11:32:46 starting to learn distance...
    	distance... 8999 (0-9000)
    2016/04/12 11:36:50 finished
    2016/04/12 11:36:50 started computing accuracy...
    	accuracy... 17999 (0-18000)
    2016/04/12 11:54:32 finished
    2016/04/12 11:54:32 Accuracy: 0.872556 0.989889


## License and funding
As far as a straight port from source code can be licensed by me (probably not at all),
the code is licensed under GPLv3.

This work is part of the [HOT research project](http://www.cs.kau.se/pulls/hot/), funded by the
 [Swedish Internet Fund](https://www.internetfonden.se/om/the-internet-fund/).
