const width = document.getElementById('graph-container').clientWidth || 1200;
const height = document.getElementById('graph-container').clientHeight || 800;

let labelsVisible = true;
const zoom = d3.zoom().on("zoom", function (event) {
    const transform = event.transform;
    g.attr("transform", transform);
    
    const shouldBeVisible = transform.k >= 0.6;
    if (shouldBeVisible !== labelsVisible) {
        labelsVisible = shouldBeVisible;
        g.selectAll('.node-label').style('opacity', shouldBeVisible ? 1 : 0);
    }
});

const svg = d3.select('#graph-container')
    .append('svg')
    .attr('width', '100%')
    .attr('height', '100%')
    .style('cursor', 'grab')
    .call(zoom)
    .on("dblclick.zoom", null); 

const g = svg.append('g');

// Zoom controls
document.getElementById('zoom-in').addEventListener('click', () => {
    svg.transition().duration(300).call(zoom.scaleBy, 1.3);
});

document.getElementById('zoom-out').addEventListener('click', () => {
    svg.transition().duration(300).call(zoom.scaleBy, 0.7);
});

document.getElementById('reset-view').addEventListener('click', () => {
    svg.transition().duration(750).call(zoom.transform, d3.zoomIdentity);
});

let nodesMap = new Map();
let linksMap = new Map();
let nodes = [];
let links = [];
let lastSelectedNode = null;

let showPhysical = true;
let showSemantic = true;

const activeClass = "bg-white dark:bg-slate-700 shadow-sm text-slate-900 dark:text-slate-100";
const inactiveClass = "text-slate-500 hover:text-slate-300";

document.getElementById('btn-physical-layer').addEventListener('click', (e) => {
    showPhysical = !showPhysical;
    const btn = e.currentTarget;
    if (showPhysical) {
        btn.className = `px-4 py-1.5 text-xs font-medium rounded-md ${activeClass} flex items-center gap-2`;
        nodes.filter(n => isNodeVisible(n) && isSemantic(n)).forEach(n => fetchAndExpandNeighborhood(n, "IMPLEMENTS"));
    } else {
        btn.className = `px-4 py-1.5 text-xs font-medium rounded-md ${inactiveClass} flex items-center gap-2`;
    }
    renderGraph();
});

document.getElementById('btn-semantic-layer').addEventListener('click', (e) => {
    showSemantic = !showSemantic;
    const btn = e.currentTarget;
    if (showSemantic) {
        btn.className = `px-4 py-1.5 text-xs font-medium rounded-md ${activeClass} flex items-center gap-2`;
        nodes.filter(n => isNodeVisible(n) && isPhysical(n)).forEach(n => fetchAndExpandNeighborhood(n, "IMPLEMENTS"));
    } else {
        btn.className = `px-4 py-1.5 text-xs font-medium rounded-md ${inactiveClass} flex items-center gap-2`;
    }
    renderGraph();
});

// Blast Radius Button (Footer)
document.getElementById('blast-radius-button').addEventListener('click', () => {
    if (lastSelectedNode) {
        runSimulation(lastSelectedNode);
    } else {
        alert("Please select a node first by clicking on it.");
    }
});

const simulation = d3.forceSimulation()
    .force("link", d3.forceLink().id(d => d.id).distance(150))
    .force("charge", d3.forceManyBody().strength(-300))
    .force("center", d3.forceCenter(width / 2, height / 2));

const nodeColors = {
    'Domain': '#4f46e5',
    'Feature': '#0891b2',
    'File': '#64748b',
    'Class': '#9333ea',
    'Interface': '#db2777',
    'Method': '#ea580c',
    'Function': '#16a34a',
    'Unknown': '#94a3b8'
};

function getColor(node) {
    const rawLabel = node.label || (node.properties && node.properties.label) || 'Unknown';
    const match = Object.keys(nodeColors).find(k => k.toLowerCase() === rawLabel.toLowerCase());
    return match ? nodeColors[match] : nodeColors['Unknown'];
}

function updateLegend() {
    const legendContainer = document.getElementById('dynamic-legend');
    if (!legendContainer) return;

    let html = `<h4 class="text-xs font-bold mb-3 uppercase tracking-wider text-slate-500">Graph Legend</h4>`;
    html += `<div class="flex flex-col gap-3">`;

    for (const [label, color] of Object.entries(nodeColors)) {
        if (label === 'Unknown') continue;
        html += `
            <div class="flex items-center gap-3">
                <div class="size-3 rounded-full" style="background-color: ${color}"></div>
                <span class="text-[10px] font-medium uppercase tracking-wide">${label}</span>
            </div>`;
    }

    html += `
        <div class="h-px bg-slate-200 dark:bg-slate-800 my-1"></div>
        <div class="flex items-center gap-3">
            <div class="size-3 rounded-full border-2 border-[#ff3366] border-dashed"></div>
            <span class="text-[10px] font-medium uppercase tracking-wide">Semantic Seam</span>
        </div>
        <div class="flex items-center gap-3">
            <div class="size-3 rounded-full border-2 border-yellow-400"></div>
            <span class="text-[10px] font-medium uppercase tracking-wide">Pinch Point</span>
        </div>
    </div>`;

    legendContainer.innerHTML = html;
}

updateLegend();

function isSemantic(n) {
    if (!n) return false;
    const label = (n.label || (n.properties && n.properties.label) || 'Node').toLowerCase();
    return label === 'domain' || label === 'feature';
}

function isPhysical(n) {
    if (!n) return false;
    return !isSemantic(n);
}

function isNodeVisible(n) {
    if (!n) return false;
    if (isSemantic(n)) return showSemantic;
    return showPhysical;
}

function isSemanticSeam(nodeId) {
    if (!semanticSeams) return false;
    for (let i = 0; i < semanticSeams.length; i++) {
        if (semanticSeams[i].method_a === nodeId || semanticSeams[i].method_b === nodeId) return true;
    }
    return false;
}

function updateGraph(newNodes, newLinks) {
    newNodes.forEach(n => {
        const id = n.id || n.Id;
        if (!id) return;
        
        if (!nodesMap.has(id)) {
            const props = n.properties || n.Properties || {};
            const normalizedNode = {
                id: id,
                name: n.name || props.name || id,
                properties: props,
                ...n
            };
            nodesMap.set(id, normalizedNode);
            nodes.push(normalizedNode);
        } else {
            // Update properties if they changed
            const existing = nodesMap.get(id);
            const props = n.properties || n.Properties || {};
            existing.properties = { ...existing.properties, ...props };
            if (n.name) existing.name = n.name;
        }
    });

    if (newLinks) {
        newLinks.forEach(l => {
            const linkId = `${l.sourceId || l.source}-${l.targetId || l.target}-${l.type}`;
            if (!linksMap.has(linkId)) {
                const link = { source: l.sourceId || l.source, target: l.targetId || l.target, type: l.type };
                linksMap.set(linkId, link);
                links.push(link);
            }
        });
    }

    renderGraph();
}

function renderGraph() {
    // Links
    let linkSelection = g.selectAll(".link")
        .data(links, d => `${d.source.id || d.source}-${d.target.id || d.target}-${d.type}`);

    linkSelection.exit().remove();

    const linkEnter = linkSelection.enter().append("line")
        .attr("class", "link")
        .attr("stroke", "#94a3b8")
        .attr("stroke-width", 2)
        .attr("stroke-opacity", 0.6)
        .attr("marker-end", "url(#arrowhead)");

    linkSelection = linkEnter.merge(linkSelection);

    linkSelection.transition().duration(500)
        .style("opacity", d => {
            const source = d.source.id ? d.source : nodesMap.get(d.source);
            const target = d.target.id ? d.target : nodesMap.get(d.target);
            return (isNodeVisible(source) && isNodeVisible(target)) ? 1 : 0.05;
        });

    // Nodes
    let nodeSelection = g.selectAll(".node-group")
        .data(nodes, d => d.id);

    nodeSelection.exit().remove();

    const nodeEnter = nodeSelection.enter()
        .append('g')
        .attr('class', 'node-group')
        .call(d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended))
        .on("click", handleNodeClick)
        .on("mouseover", handleNodeMouseOver)
        .on("mouseout", handleNodeMouseOut)
        .on("dblclick", handleNodeDoubleClick)
        .on("contextmenu", handleNodeContextMenu);

    nodeEnter.append('circle')
        .attr('class', 'node')
        .attr('r', 20)
        .attr('fill', d => getColor(d))
        .attr('stroke', '#fff')
        .attr('stroke-width', 2);

    nodeEnter.append('text')
        .attr('class', 'node-label')
        .attr('dy', 30)
        .attr('text-anchor', 'middle')
        .style('opacity', labelsVisible ? 1 : 0)
        .text(d => d.name || d.id);

    nodeSelection = nodeEnter.merge(nodeSelection);
    
    nodeSelection.transition().duration(500)
        .style("opacity", d => isNodeVisible(d) ? 1 : 0.1)
        .style("pointer-events", d => isNodeVisible(d) ? "auto" : "none");

    nodeSelection.select('circle')
        .attr('class', d => {
            let cls = 'node';
            if (showPinchPoints && pinchPoints.has(d.id)) cls += ' pinch-point';
            if (showSemanticSeams && isSemanticSeam(d.id)) cls += ' semantic-seam';
            return cls;
        });

    simulation.nodes(nodes);
    simulation.force("link").links(links);
    simulation.alpha(1).restart();

    simulation.on("tick", () => {
        g.selectAll(".link")
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        g.selectAll(".node-group")
            .attr("transform", d => `translate(${d.x},${d.y})`);
    });
}

function dragstarted(event, d) {
    if (!event.active) simulation.alphaTarget(0.3).restart();
    d.fx = d.x;
    d.fy = d.y;
}

function dragged(event, d) {
    d.fx = event.x;
    d.fy = event.y;
}

function dragended(event, d) {
    if (!event.active) simulation.alphaTarget(0);
    d.fx = null;
    d.fy = null;
}

async function handleNodeClick(event, d) {
    console.log("Node clicked:", d.id);
    lastSelectedNode = d;
    showNodeDetails(d);
    await fetchAndExpandNeighborhood(d);
}

function handleNodeMouseOver(event, d) {
    d3.select(this).select('circle')
        .transition().duration(200)
        .attr('r', 25)
        .attr('stroke-width', 4);
    showNodeDetails(d);
}

function handleNodeMouseOut(event, d) {
    d3.select(this).select('circle')
        .transition().duration(200)
        .attr('r', 20)
        .attr('stroke-width', 2);
}

function showNodeDetails(d) {
    const panel = document.getElementById('impact-panel');
    if (!panel) return;
    panel.style.display = 'flex';
    
    document.getElementById('impact-node-name').textContent = d.name || d.id;
    
    const typeEl = document.getElementById('impact-node-type');
    if (typeEl) {
        typeEl.textContent = d.label || (d.properties && d.properties.label) || 'Node';
    }
    
    const placeholder = document.getElementById('impact-placeholder');
    if (placeholder) placeholder.classList.add('hidden');
    
    const details = document.getElementById('impact-details');
    if (details) details.classList.remove('hidden');
    
    const props = d.properties || d.Properties || {};
    const riskScore = props.volatility_score || 0;

    const maxRisk = nodes.reduce((max, node) => {
        const score = (node.properties && node.properties.volatility_score) || 0;
        return score > max ? score : max;
    }, 0.0001);

    const riskPercent = Math.min(100, Math.round((riskScore / maxRisk) * 100));
    
    const riskScoreEl = document.getElementById('risk-score');
    if (riskScoreEl) riskScoreEl.textContent = `${riskPercent}/100`;
    
    const riskBarEl = document.getElementById('risk-bar');
    if (riskBarEl) riskBarEl.style.width = `${riskPercent}%`;
    
    let riskLabel = 'LOW';
    if (riskPercent > 70) riskLabel = 'CRITICAL';
    else if (riskPercent > 40) riskLabel = 'MEDIUM';
    
    const riskLabelEl = document.getElementById('risk-label');
    if (riskLabelEl) riskLabelEl.textContent = riskLabel;
    
    const propsContainer = document.getElementById('impact-properties');
    if (propsContainer) {
        propsContainer.innerHTML = '';
        for (const [key, value] of Object.entries(props)) {
            if (key === 'name' || key === 'id') continue;
            const row = document.createElement('div');
            row.className = 'grid grid-cols-3 gap-2 border-b border-slate-700/50 pb-1 mb-1';
            const displayValue = String(value).length > 30 ? String(value).substring(0, 27) + '...' : value;
            row.innerHTML = `<span class="text-slate-500 font-medium capitalize truncate" title="${key}">${key}</span><span class="col-span-2 truncate text-slate-300" title="${value}">${displayValue}</span>`;
            propsContainer.appendChild(row);
        }
    }
}

async function handleNodeDoubleClick(event, d) {
    console.log("Node double-clicked:", d.id);
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }

    const currentWidth = document.getElementById('graph-container').clientWidth || width;
    const currentHeight = document.getElementById('graph-container').clientHeight || height;

    d.fx = d.x;
    d.fy = d.y;

    const scale = 2.0;
    const transform = d3.zoomIdentity
        .translate(currentWidth / 2, currentHeight / 2)
        .scale(scale)
        .translate(-d.x, -d.y);

    svg.transition()
        .duration(750)
        .call(zoom.transform, transform);

    await fetchAndExpandNeighborhood(d);
    
    setTimeout(() => {
        d.fx = null;
        d.fy = null;
    }, 1500);
}

let contextNode = null;
const contextMenu = document.getElementById('context-menu');

function handleNodeContextMenu(event, d) {
    event.preventDefault();
    contextNode = d;
    contextMenu.style.display = 'block';
    contextMenu.style.left = `${event.pageX}px`;
    contextMenu.style.top = `${event.pageY}px`;
}

document.addEventListener('click', () => {
    if (contextMenu) contextMenu.style.display = 'none';
});

async function runSimulation(node) {
    if (!node) return;
    const target = node.id;
    
    showNodeDetails(node);
    
    const riskLabelEl = document.getElementById('risk-label');
    if (riskLabelEl) riskLabelEl.textContent = 'ANALYZING...';
    
    try {
        const response = await fetch(`/api/query?type=what-if&target=${encodeURIComponent(target)}`);
        const data = await response.json();
        
        const impactedNodesCount = (data.orphaned_nodes?.length || 0) + (data.severed_edges?.length || 0);
        const statNodes = document.getElementById('stat-nodes');
        if (statNodes) statNodes.textContent = impactedNodesCount;
        
        const statDepth = document.getElementById('stat-depth');
        if (statDepth) statDepth.textContent = impactedNodesCount > 0 ? '1+' : '0';
        
        const vizContainer = document.getElementById('impact-visualizations') || document.createElement('div');
        vizContainer.id = 'impact-visualizations';
        const impactDetails = document.getElementById('impact-details');
        if (impactDetails && !document.getElementById('impact-visualizations')) {
            impactDetails.appendChild(vizContainer);
        }
        
        vizContainer.innerHTML = '<h4 class="text-[10px] font-bold uppercase tracking-widest text-slate-500 mt-4 mb-2">Affected Pathways</h4>';

        if (data && (data.severed_edges?.length > 0 || data.orphaned_nodes?.length > 0)) {
            (data.orphaned_nodes || []).forEach(node => {
                const item = document.createElement('div');
                item.className = 'flex items-center justify-between p-2 rounded-lg bg-slate-100 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 mb-2';
                item.innerHTML = `
                    <div class="flex items-center gap-3">
                        <span class="material-symbols-outlined text-impact-high text-lg">dangerous</span>
                        <div class="flex flex-col">
                            <span class="text-xs font-bold">${node.id}</span>
                            <span class="text-[10px] text-slate-500">Orphaned</span>
                        </div>
                    </div>
                `;
                vizContainer.appendChild(item);
            });

            (data.severed_edges || []).forEach(edge => {
                const item = document.createElement('div');
                item.className = 'flex items-center justify-between p-2 rounded-lg bg-slate-100 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 mb-2';
                item.innerHTML = `
                    <div class="flex items-center gap-3">
                        <span class="material-symbols-outlined text-impact-medium text-lg">link_off</span>
                        <div class="flex flex-col">
                            <span class="text-xs font-bold">${edge.targetId}</span>
                            <span class="text-[10px] text-slate-500">Severed from ${edge.sourceId}</span>
                        </div>
                    </div>
                `;
                vizContainer.appendChild(item);
            });
            
            if (riskLabelEl) {
                if (impactedNodesCount > 10) riskLabelEl.textContent = 'CRITICAL';
                else if (impactedNodesCount > 0) riskLabelEl.textContent = 'HIGH';
                else riskLabelEl.textContent = 'STABLE';
            }
        } else {
            vizContainer.innerHTML += `<p class="text-xs text-slate-500 italic">No significant impact found.</p>`;
            if (riskLabelEl) riskLabelEl.textContent = 'STABLE';
        }
    } catch (err) {
        console.error("Failed to simulate extraction:", err);
        if (riskLabelEl) riskLabelEl.textContent = 'ERROR';
    }
}

document.getElementById('menu-simulate-extraction').addEventListener('click', () => {
    if (contextNode) runSimulation(contextNode);
});

svg.append("defs").append("marker")
    .attr("id", "arrowhead")
    .attr("viewBox", "0 -5 10 10")
    .attr("refX", 25)
    .attr("refY", 0)
    .attr("orient", "auto")
    .attr("markerWidth", 6)
    .attr("markerHeight", 6)
    .attr("xoverflow", "visible")
    .append("svg:path")
    .attr("d", "M 0,-5 L 10 ,0 L 0,5")
    .attr("fill", "#999")
    .style("stroke", "none");

document.getElementById('search-button').addEventListener('click', async () => {
    const target = document.getElementById('search-input').value;
    if (!target) return;

    try {
        const response = await fetch(`/api/query?type=search-similar&target=${encodeURIComponent(target)}`);
        const data = await response.json();
        
        if (data && Array.isArray(data)) {
            let fetchedNodes = data.map(item => {
                const node = item.node || item;
                return {
                    id: node.id || node.Id,
                    label: node.label || node.Label || 'Node',
                    name: (node.properties && node.properties.name) || (node.Properties && node.Properties.name) || node.name || node.id || node.Id,
                    properties: node.properties || node.Properties,
                    ...item
                };
            });
            
            nodesMap.clear();
            linksMap.clear();
            nodes = [];
            links = [];
            
            updateGraph(fetchedNodes, []);
            
            setTimeout(() => {
                svg.transition().duration(750).call(zoom.transform, d3.zoomIdentity);
            }, 500);
        }
    } catch (err) {
        console.error("Failed to fetch data:", err);
    }
});

async function loadInitialOverview() {
    try {
        const response = await fetch('/api/query?type=overview');
        const data = await response.json();
        
        if (data && data.nodes) {
            const fetchedNodes = data.nodes.map(n => ({
                id: n.id || n.Id,
                name: (n.properties && n.properties.name) || (n.Properties && n.Properties.name) || n.id || n.Id,
                properties: n.properties || n.Properties,
                ...n
            }));
            
            nodesMap.clear();
            linksMap.clear();
            nodes = [];
            links = [];
            updateGraph(fetchedNodes, data.edges || []);
        }
    } catch (err) {
        console.error("Failed to fetch initial overview:", err);
    }
}

loadInitialOverview();

let showPinchPoints = false;
let showSemanticSeams = false;
let pinchPoints = new Set();
let semanticSeams = [];

document.getElementById('pinch-points-button').addEventListener('click', async (e) => {
    showPinchPoints = !showPinchPoints;
    const btn = e.currentTarget;
    btn.classList.toggle('bg-primary/20', showPinchPoints);
    btn.classList.toggle('border-primary', showPinchPoints);
    
    if (showPinchPoints && pinchPoints.size === 0) {
        try {
            const res = await fetch('/api/query?type=seams');
            const data = await res.json();
            if (Array.isArray(data)) {
                data.forEach(s => pinchPoints.add(s.seam || s.Seam));
            }
        } catch (err) { console.error(err); }
    }
    renderGraph();
});

document.getElementById('semantic-seams-button').addEventListener('click', async (e) => {
    showSemanticSeams = !showSemanticSeams;
    const btn = e.currentTarget;
    btn.classList.toggle('bg-primary/20', showSemanticSeams);
    btn.classList.toggle('border-primary', showSemanticSeams);
    
    if (showSemanticSeams && semanticSeams.length === 0) {
        try {
            const res = await fetch('/api/query?type=semantic-seams&similarity=0.6');
            const data = await res.json();
            if (Array.isArray(data)) {
                semanticSeams = data;
            }
        } catch (err) { console.error(err); }
    }
    renderGraph();
});

// Initialize side panel buttons
function initSidePanelButtons() {
    const btnExpand = document.getElementById('btn-expand-node');
    if (btnExpand) {
        btnExpand.addEventListener('click', () => {
            if (lastSelectedNode) handleNodeDoubleClick(null, lastSelectedNode);
        });
    }

    const btnTrace = document.getElementById('btn-trace-intent');
    if (btnTrace) {
        btnTrace.addEventListener('click', async () => {
            if (!lastSelectedNode) return;
            try {
                const response = await fetch(`/api/query?type=semantic-trace&target=${encodeURIComponent(lastSelectedNode.id)}`);
                const data = await response.json();
                
                if (data && Array.isArray(data)) {
                    let fetchedNodes = [];
                    let fetchedLinks = [];
                    data.forEach(path => {
                        if (path.nodes) fetchedNodes.push(...path.nodes.map(n => ({
                            id: n.id || n.Id,
                            label: n.label || n.Label || 'Node',
                            name: (n.properties && n.properties.name) || (n.Properties && n.Properties.name) || n.name || n.id || n.Id,
                            properties: n.properties || n.Properties
                        })));
                        if (path.edges) fetchedLinks.push(...path.edges.map(e => ({
                            source: e.sourceId || e.SourceID,
                            target: e.targetId || e.TargetID,
                            type: e.type || e.Type
                        })));
                    });
                    updateGraph(fetchedNodes, fetchedLinks);
                }
            } catch (err) { console.error(err); }
        });
    }

    const btnSimulate = document.getElementById('btn-simulate-extraction');
    if (btnSimulate) {
        btnSimulate.addEventListener('click', () => {
            if (lastSelectedNode) runSimulation(lastSelectedNode);
        });
    }
}

initSidePanelButtons();

async function fetchAndExpandNeighborhood(d, edgeTypes = "") {
    try {
        let url = `/api/query?type=traverse&target=${encodeURIComponent(d.id)}&direction=both&depth=1`;
        if (edgeTypes) url += `&edge-types=${encodeURIComponent(edgeTypes)}`;
        const response = await fetch(url);
        const data = await response.json();
        
        if (data && Array.isArray(data)) {
            let fetchedNodes = [];
            let fetchedLinks = [];
            data.forEach(path => {
                if (path.nodes) fetchedNodes.push(...path.nodes.map(n => ({
                    id: n.id || n.Id,
                    label: n.label || n.Label || 'Node',
                    name: (n.properties && n.properties.name) || (n.Properties && n.Properties.name) || n.name || n.id || n.Id,
                    properties: n.properties || n.Properties
                })));
                if (path.edges) fetchedLinks.push(...path.edges.map(e => ({
                    source: e.sourceId || e.SourceID,
                    target: e.targetId || e.TargetID,
                    type: e.type || e.Type
                })));
            });
            updateGraph(fetchedNodes, fetchedLinks);
        }
    } catch (err) { console.error(err); }
}

console.log("GraphDB Visualizer initialized. Version: " + new Date().toISOString());
