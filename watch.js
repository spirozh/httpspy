
const evtSource = new EventSource("/SSEUpdate");

let qurl = "";

evtSource.onmessage = (event) => {
    refreshRequests();
}

function changeTable(requestsBody) {
    requests = JSON.parse(requestsBody);

    var fragment = $('<table></table>');

    const cols = ['Id', 'Timestamp', 'Method', 'URL', 'Body'];

    var tr = $(`<tr></tr>`);
    cols.forEach(col => {
        let th = $(`<th></th>`);
        let div = $(`<div>${col}</div>`)
        if (col === 'URL') {
            if (qurl !== '') {
                th = $(`<th class="clickable"></th>`);
                th.on('click', () => { qurl = ""; refreshRequests(); });
            }
        }
        th.append(div);
        tr.append(th);
    });
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
    let div = $(`<div></div>`);

    let text = "";
    switch (col) {
        case 'Timestamp':
            text = request[col].replace('T', ' ');
            break;
        case 'URL':
            if (qurl === "") {
                td.on('click', () => { qurl = request[col]; refreshRequests() });
                td.attr('class', (i, v) => `${v} clickable`);
            }
        default:
            text = request[col];
    }
    div.append(request[col])

    td.append(div);
    return td;
}

function refreshRequests() {
    $.get('/requests?url=' + encodeURIComponent(qurl), changeTable);
}

$("button#clear").click(() => {
    if (window.confirm("are you sure?"))
        $.get("clear");
})
