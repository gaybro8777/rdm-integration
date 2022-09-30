var myTree;

function getSelected(fName) {
    var values = [];
    var cbs = document.forms[fName].elements['files'];
    if (!cbs) {
        return values;
    }
    if (cbs.checked) { //if only one checkbox in the form
        values.push(cbs.value);
        return values;
    }
    for (var i = 0, cbLen = cbs.length; i < cbLen; i++) {
        if (cbs[i].checked) {
            values.push(cbs[i].value);
        }
    }
    return values;
}

async function showConfirmationDialog() {
    if (!myTree) {
        return
    }
    let x = document.getElementById("frm1");
    let data = {
        selectedNodes: myTree.selectedNodes,
        originalRoot: res,
    };
    let fetched = await fetch("../../api/common/writable", {
        method: "POST",
        body: JSON.stringify(data),
    })
    if (fetched.status != 200) {
        alert(await fetched.text());
        document.getElementById("confirmation").innerHTML = '';
    } else {
        let toConfirm = await fetched.json()
        showConfirmation(toConfirm);
    }
}

function showConfirmation(toConfirm) {
    document.getElementById("instructions").innerHTML = '<span style="font-weight: bold;">Select the files to <span style="font-weight: 900; color: red;">KEEP</span> in Dataverse:<br/><br/></span><button onclick="myTree.collapseAll()">Collaps all</button>';
    document.getElementById("legend").innerHTML = `
    Legend:
        <ul>
            <li><span style="color: green;">Files only at remote location</span></li>
            <li><span style="color: black;">The same version is at remote locaction as in Dataverse</span></li>
            <li><span style="color: blue;">Remote location version does not match Dataverse version</span></li>
            <li><span style="color: gray;">Files only in Dataverse</span></li>
        </ul>
    `;
    document.getElementById("frm1").style.display = 'none';
    if (toConfirm.toUpdate.length == 0 && toConfirm.toDelete.length == 0 && toConfirm.toAdd.length == 0) {
        document.getElementById("confirmation").innerHTML = 'Nothing to update, add or to delete...<br/><br/><button onclick="cancel()">Cancel</button>';
        return;
    }

    let form = '';
    if (toConfirm.toDelete.length != 0) {
        form += '<span style="font-weight: bold;">Files that will be <span style="font-weight: 900; color: red;">DELETED</span> from Dataverse:</span><form name="toDelete"><br/>';
        for (let i = 0, l = toConfirm.toDelete.length; i < l; i++) {
            form += toCheckbox(toConfirm.toDelete[i], 'red');
        }
        form += '</form><br/>';
    } else {
        form += '<form name="toDelete"></form>';
    }

    if (toConfirm.toAdd.length != 0) {
        form += '<span style="font-weight: bold;">Files that will be <span style="font-weight: 900; color: green;">ADDED</span> to Dataverse:</span><form name="toAdd"><br/>';
        for (let i = 0, l = toConfirm.toAdd.length; i < l; i++) {
            form += toCheckbox(toConfirm.toAdd[i], 'green');
        }
        form += '</form><br/>';
    } else {
        form += '<form name="toAdd"></form>';
    }

    if (toConfirm.toUpdate.length != 0) {
        form += '<span style="font-weight: bold;">Files that will be <span style="font-weight: 900; color: blue;">UPDATED</span> in Dataverse:</span><form name="toUpdate"><br/>';
        for (let i = 0, l = toConfirm.toUpdate.length; i < l; i++) {
            form += toCheckbox(toConfirm.toUpdate[i], 'blue');
        }
        form += '</form><br/>';
    } else {
        form += '<form name="toUpdate"></form>';
    }
    
    form += '<span><br/></span><button onclick="store()">OK</button><button onclick="cancel()">Cancel</button><br/><br/>';

    document.getElementById("confirmation").innerHTML = form;
}

function toCheckbox(value, color) {
    return '<p><input type="checkbox" name="files" value="' + value + '" checked="true"/><span style="color: ' + color + ';">' + value + '</span></p>';
}

function cancel() {
    document.getElementById("content").innerHTML = '';
    document.getElementById("instructions").innerHTML = '';
    document.getElementById("legend").innerHTML = '';
    document.getElementById("confirmation").innerHTML = '';
    document.getElementById("frm1").style.display = 'block';
}
