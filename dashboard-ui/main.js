
const listAppPods = () => {
    fetch('/api/v1/pods')
        .then(response => response.json())
        .then(pods => {
            let podsList = document.getElementById("podsList");
            podsList.innerHTML = "";
            pods.forEach(pod => {
                podsList.innerHTML += `<li>${pod.metadata.name}</li>`;
            });
        });
}

const listMountPods = () => {
    fetch('/api/v1/mountpods')
        .then(response => response.json())
        .then(pods => {
            let podsList = document.getElementById("podsList");
            podsList.innerHTML = "";
            pods.forEach(pod => {
                podsList.innerHTML += `<li>${pod.metadata.name}</li>`;
            });
        });
}

const listCSINodePods = () => {
    fetch('/api/v1/csi-nodes')
        .then(response => response.json())
        .then(pods => {
            let podsList = document.getElementById("podsList");
            podsList.innerHTML = "";
            pods.forEach(pod => {
                podsList.innerHTML += `<li>${pod.metadata.name}</li>`;
            });
        });
}

const listControllerPods = () => {
    fetch('/api/v1/controllers')
        .then(response => response.json())
        .then(pods => {
            let podsList = document.getElementById("podsList");
            podsList.innerHTML = "";
            pods.forEach(pod => {
                podsList.innerHTML += `<li>${pod.metadata.name}</li>`;
            });
        });
}