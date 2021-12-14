---
sidebar_label: Build JuiceFS CSI Driver Image 
---

# How to build JuiceFS CSI Driver image by yourself

If you need to modify the JuiceFS code yourself and build the CSI Driver image, you can follow the steps below.

Clone the JuiceFS repository and modify the code as needed:

```shell
git clone git@github.com:juicedata/juicefs.git
```

Copy the [dev.juicefs.Dockerfile](https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/dev.juicefs.Dockerfile) file in the JuiceFS CSI Driver repository to the path you just cloned, and execute the following command to build the image:

```bash
docker build -f dev.juicefs.Dockerfile .
```
