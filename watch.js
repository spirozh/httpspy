
const evtSource = new EventSource("/SSEUpdate");

evtSource.onmessage = (event) => {
    refreshRequests();
}

function changeTable(requestsBody) {
    requests = JSON.parse(requestsBody);

    var fragment = $('<table></table>');

    const cols = ['Id', 'Timestamp', 'Method', 'URL', 'Body'];

    var tr = $(`<tr></tr>`);
    cols.forEach(col => { tr.append($(`<th>${col}</th>`)); });
    fragment.append(tr);

    if (!requests) {
        fragment.append($(`<tr><td colspan="${cols.length}"><div class="no-requests">no requests</div></td></tr>`))
    } else
        requests.forEach(r => {
            var tr = $('<tr></tr>');
            cols.forEach(col => tr.append(renderCell(r, col)));
            fragment.append(tr);
        });


    $('div#requests table').remove();
    $('div#requests').append(fragment);
}

function renderCell(request, col) {
    let td = $(`<td class="${col}"></td>`);

    switch (col) {
        case 'Timestamp':
            td.append(request[col].replace('T', ' '));
            break;
        case 'Headers':
            var headers = JSON.parse(request[col]);
            Object.entries(headers).forEach(e => {
                const [k, v] = e;
                if (k.startsWith("Sec")) return;
                td.append($(`<span class="tt">${k}<span class="ttt">${v}</span></span> `));
            });
            break;
        default:
            td.append(request[col])
    }

    return td;
}

function refreshRequests() {
    $.get("requests", changeTable)
}

$("button#clear").click(() => {
    if (window.confirm("are you sure?"))
        $.get("clear");
})
