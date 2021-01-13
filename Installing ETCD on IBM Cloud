### Installing ETCD on IBM Cloud
This document will describe how to install ETCD on IBM Cloud using Kubernetes services.

### Contents

1. Introduction
2. Provision Kubernetes Cluster
3. Deploy IBM Cloud Block-Storage Plugin
4. Deploy ETCD
5. Verifying the ETCD Installation

### Introduction
To complete this tutorial, you should have an IBM Cloud account, if you do not have one, please [register/signup here](https://cloud.ibm.com/registration).
For installing ETCD, we have used the Kubernetes cluster, and used the IBM Cloud Block-Storage plugin for our persistent volume. Upon the completion of this tutorial, you would have the ETCD up and running on the Kubernetes cluster.

1. Provision the Kubernetes cluster, if you have already setup one, skip to step 2.
2. Deploy the IBM Cloud Block-Storage Plugin to the created cluster, if you have already done this, skip to step 3.
3. Deploy the ETCD.

### Provision Kubernetes Cluster
* Click on the **Catalog** button on top center. Open [Catalog](https://cloud.ibm.com/catalog). ![alt text](Kubernetes1.png)

* In search catalog box, search for **Kubernetes Service** and click on it. ![alt text](Kubernetes2.png)

* You are now at Create Kubernetes Cluster page, there you have the two plans to create the Kubernetes cluster, either using **free plan** or **standard plan**.

**Option1- Using Free Plan:**
* Select Pricing Plan as “**Free**”. ![alt text](Kubernetes3.png)
* Click on **Create**.
* Wait a few minutes, and then your Cloud would be ready.

>**Note**: _Please be careful when choosing free cluster, as your pods could be stuck at pending state due to insufficient compute and memory resources, if you face such kind of issue please increase your resource by creating them choosing the standard plan._

**Option2- Using Standard Plan:**
* Select Pricing Plan as “**Standard**” 
* Select your **Kubernetes Version** as latest available or desired one by application. In our example we have set it to be '**1.18.13**'. 

  ![alt text](Kubernetes4.png)

* Select Infrastructure as “**Classic**”

  ![alt text](Kubernetes5.png)

* Leave Resource Group to “**Default**”
* Select Geography as “**Asia**” or your desired one.
* Select Availability as “**Single Zone**”.
>_This option allows you to create the resources in either single or multi availability zones. Multi availability zone provides you the option to create the resources in more than one availability zones so in case of catastrophe, it could sustain the disaster and continues to work._
* Select Worker Zone as **Chennai 01**.
* In Worker Pool, input your desired number of nodes as “**2**” ![alt text](Kubernetes6.png)
* Leave the Encrypt Local Disk option to “**On**”
* Select Master Service Endpoint to “**Both private and public endpoints**”
* Give your cluster-name as “**ETCD-Cluster**”
* Provide tags to your cluster and click on **Create**.
* Wait a few minutes, and then your Cloud would be ready.

  ![alt text](Kubernetes7.png)

### Deploy IBM Cloud Block-Storage Plugin
* Click on the **Catalog** button on top center.
* In search catalog box, search for **IBM Cloud Block Storage Plug-In** and click on it ![alt text](Storage1.png)
* Select your cluster as "**ETCD-Cluster**"
* Provide Target Namespace as “**etcd-storage**”, leave name and resource group to **default values**. ![alt text](Storage2.png)
* Click on Install

### Deploy ETCD
* Again go to the **Catalog** and search for **ETCD**. 
  
  ![alt text](ETCD1.png)
  
* Provide the details as below. ![alt text](ETCD2.png)

*	Target: **IBM Kubernetes Service**
*	Method: **Helm chart**
*	Kubernetes cluster: **ETCD-Cluster(jp-tok)**
*	Target namespace: **etcd**
*	Workspace: **etcd-01-06-2021**
*	Resource group: **Default**
*  Click on **Additional Parameters** with **Default Values**, you can set the deployment values or use the default ones, we have used the default ones except for setting the **Auth. Root Password**.

* Click on **Install**.

### Verifying the ETCD Installation
* Go to the **Resources List** in the Left Navigation Menu and click on **Kubernetes** and then **Clusters** 
* Click on your created **ETCD-Cluster**. 

  ![alt text](ETCD3.png)
  
* A screen would come up for your created cluster, click on **Actions**, and then **Web Terminal** ![alt text](ETCDVerify1.png)
* A warning will appear asking you to install the Web Terminal, click on **Install**.
* When the terminal is installed, click on the action button again and click on web terminal and type the following command in below command window. It will show you the workspaces of your cluster, you can see ETCD active.

```sh
     $ kubectl get ns 
```
![alt text](ETCDVerify2.png)
     
```sh  
     $ kubectl get pod –n Namespace –o wide 
```
![alt text](ETCDVerify3.png)

```sh  
     $ kubectl get service –n Namespace 
```
![alt text](ETCDVerify4.png)

```sh
     $ kubectl exec --stdin --tty PODNAME -n NAMESPACE -- /bin/bash 
     $ etcdctl --help 
```
![alt text](ETCDVerify5.png)


The installation is done. Enjoy!
