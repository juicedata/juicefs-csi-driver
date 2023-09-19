const renderPodsList = (pods) => {
    let podsList = document.getElementById("podsList");
    podsList.innerHTML = "";
    pods.forEach(pod => {
        podsList.innerHTML += `<li>${pod.metadata.name}</li>`;
    });
}


const listAppPods = () => {
    fetch('/api/v1/pods')
        .then(response => response.json())
        .then(renderPodsList);
}

const listMountPods = () => {
    fetch('/api/v1/mountpods')
        .then(response => response.json())
        .then(renderPodsList);
}

const listCSINodePods = () => {
    fetch('/api/v1/csi-nodes')
        .then(response => response.json())
        .then(renderPodsList);
}

const listControllerPods = () => {
    fetch('/api/v1/controllers')
        .then(response => response.json())
        .then(renderPodsList);
}
