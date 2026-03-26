import { nodeColors } from './config.js';
import { nodes, nodesMap, visibilitySettings } from './state.js';

export function getColor(node) {
    let rawLabel = node.label || 'Unknown';
    if ((rawLabel === 'CodeElement' || rawLabel === 'Unknown') && node.properties && node.properties.node_label) {
        rawLabel = node.properties.node_label;
    }
    const match = Object.keys(nodeColors).find(k => k.toLowerCase() === rawLabel.toLowerCase());
    return match ? nodeColors[match] : nodeColors['Unknown'];
}

export function updateLegend() {
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

export function isSemantic(n) {
    if (!n) return false;
    let label = (n.label || 'Node').toLowerCase();
    if (n.properties && n.properties.node_label) {
        label = n.properties.node_label.toLowerCase();
    } else if (n.properties && n.properties.label) {
        label = n.properties.label.toLowerCase();
    }
    return label === 'domain' || label === 'feature';
}

export function isPhysical(n) {
    if (!n) return false;
    return !isSemantic(n);
}

export function isNodeVisible(n) {
    if (!n) return false;
    if (isSemantic(n)) return visibilitySettings.showSemantic;
    return visibilitySettings.showPhysical;
}

export function resolveNodeName(nodeId) {
    const node = nodesMap.get(nodeId);
    if (node) {
        return node.name || (node.properties && node.properties.name) || nodeId;
    }
    return nodeId;
}

export function togglePanel(id, show) {
    const panel = document.getElementById(id);
    if (!panel) return;
    if (show === undefined) {
        panel.style.display = panel.style.display === 'none' ? 'flex' : 'none';
    } else {
        panel.style.display = show ? 'flex' : 'none';
    }
}

export function showNodeDetails(d) {
    const panel = document.getElementById('impact-panel');
    if (!panel) return;
    panel.style.display = 'flex';
    
    const rawName = d.name || d.id;
    let displayName = rawName;
    if (rawName && rawName.length > 40) {
        if (rawName.includes('/') || rawName.includes('\\') || rawName.includes('.') || rawName.includes(':')) {
            displayName = '...' + rawName.substring(rawName.length - 37);
        }
    }
    const nameEl = document.getElementById('impact-node-name');
    nameEl.textContent = displayName;
    nameEl.title = rawName;
    
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
        
        // Update risk description if it exists in props
        const riskDescEl = document.getElementById('risk-description');
        if (riskDescEl) {
            if (props.description) {
                riskDescEl.textContent = props.description;
                riskDescEl.classList.remove('italic');
            } else {
                riskDescEl.textContent = 'No description available for this component.';
                riskDescEl.classList.add('italic');
            }
        }

        for (const [key, value] of Object.entries(props)) {
            if (key === 'name' || key === 'id' || key === 'description') continue;
            const row = document.createElement('div');
            row.className = 'grid grid-cols-3 gap-2 border-b border-slate-700/50 pb-1 mb-1';
            
            let displayValue = value;
            // Format numeric scores as percentages
            if (typeof value === 'number' && (key.includes('score') || key.includes('risk') || key.includes('volatility'))) {
                displayValue = (value * 100).toFixed(1) + '%';
            } else if (String(value).length > 30) {
                const lowerKey = key.toLowerCase();
                if (lowerKey === 'file' || lowerKey === 'fqn' || lowerKey === 'path' || lowerKey === 'module' || lowerKey === 'full_name') {
                    displayValue = '...' + String(value).substring(String(value).length - 27);
                } else {
                    displayValue = String(value).substring(0, 27) + '...';
                }
            }
            
            row.innerHTML = `<span class="text-slate-500 font-medium capitalize truncate" title="${key}">${key.replace(/_/g, ' ')}</span><span class="col-span-2 truncate text-slate-300" title="${value}">${displayValue}</span>`;
            propsContainer.appendChild(row);
        }

        // Add description at the end if it exists, full width
        if (props.description) {
            const descRow = document.createElement('div');
            descRow.className = 'flex flex-col gap-1 border-b border-slate-700/50 pb-2 mb-1 mt-2';
            descRow.innerHTML = `
                <span class="text-slate-500 font-medium capitalize text-[10px] uppercase tracking-wider">Description</span>
                <span class="text-slate-300 text-xs leading-relaxed whitespace-pre-wrap">${props.description}</span>
            `;
            propsContainer.appendChild(descRow);
        }
    }

    const btnExpand = document.getElementById('btn-expand-node');
    if (btnExpand) {
        if (d._expanded) {
            btnExpand.innerHTML = '<span class="material-symbols-outlined text-[14px]">close_fullscreen</span> Collapse Relationships';
        } else {
            btnExpand.innerHTML = '<span class="material-symbols-outlined text-[14px]">open_in_full</span> Expand Relationships';
        }
    }
}

