let qurl = '';
let requests;

const sse = new EventSource('/SSEUpdate');
sse.addEventListener('all', e => {
    requests = JSON.parse(e.data)
    updateTable();
});
sse.addEventListener('new', e => {
    let newR = JSON.parse(e.data);
    requests.push(newR);

    if (qurl === '' || qurl === newR.URL){
        updateTable();
    }
});
sse.addEventListener('clear', e => {
    requests = JSON.parse(e.data);
    updateTable();
});

function updateTable() {
    updateURLHeader();
    updateTBody();
}

qurl = '';
function updateURLHeader() {
    let elt = $('#URLHeader');
    elt.empty()
    if (qurl === '') {
        elt.append($('<div>URL</div>'))
    } else {
        elt.append($('<a href onclick="qurl=\'\'; updateTable(); return false;">URL</a>'))
    }
}

function updateTBody() {
    var fragment = $('<tbody id="tableBody"></tbody>');

    const cols = ['ID', 'Timestamp', 'Method', 'URL', 'Body'];

    if (requests.length === 0) {
        fragment.append($(`<tr><td colspan="${cols.length}"><div class="no-requests">no requests</div></td></tr>`))
    } else
        requests.toReversed().filter( r => qurl === '' || qurl === r.URL ).forEach(r => {
            var tr = $('<tr></tr>');
            cols.forEach(col => tr.append(renderCell(r, col)));
            fragment.append(tr);
        });

    let elt = $('#tableBody');
    elt.replaceWith(fragment);
}

function renderCell(request, col) {
    let td = $(`<td class="${col}"></td>`);
    let div = $(`<div></div>`);

    let text = request[col];
    switch (col) {
        case 'Timestamp':
            text = text.replace('T', ' ');
            break;
        case 'URL':
            if (qurl === "") {
                text = $(`<a href onclick="qurl='${text}'; updateTable(); return false;">${text}</a>`);
            }
            break;
        default:
            text = request[col];
    }
    div.append(text)

    td.append(div);
    return td;
}

$("button#clear").click(() => {
    if (window.confirm("are you sure?"))
        $.get("clear");
})
