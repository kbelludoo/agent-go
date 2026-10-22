import json
import gzip
import base64
from pathlib import Path

# Load extracted game data
with open("kaggle/real_multigame_extracted.json", "r", encoding="utf-8") as f:
    data = json.load(f)

compressed_b64 = base64.b64encode(gzip.compress(json.dumps(data).encode("utf-8"))).decode("ascii")

# Load base build_bot_brain.py lines from FASE 1 onward
with open("kaggle/build_bot_brain.py", "r", encoding="utf-8") as f:
    lines = f.readlines()

# Find where FASE 1 starts
start_idx = 0
for idx, line in enumerate(lines):
    if "FASE 1: REDE NEURAL" in line:
        start_idx = idx - 2
        break

rest_of_code = "".join(lines[start_idx:])

# Also optimize Phase 4 to pass numpy array directly instead of emb.tolist()
rest_of_code = rest_of_code.replace(
    '    for i, emb in enumerate(embeddings):\n        chunks[i]["vector"] = emb.tolist()\n    return dim',
    '    return embeddings, dim'
)

rest_of_code = rest_of_code.replace(
    'def export_master_artifacts(chunks, dim):',
    'def export_master_artifacts(chunks, embeddings, dim):'
)

rest_of_code = rest_of_code.replace(
    '''            vec = c["vector"]
            f.write(struct.pack("<H", len(vec)))
            for val in vec:
                f.write(struct.pack("<f", float(val)))''',
    '''            emb_bytes = embeddings[i].astype(np.float32).tobytes()
            f.write(struct.pack("<H", dim))
            f.write(emb_bytes)'''
)

rest_of_code = rest_of_code.replace(
    '''    # 4. Vetorização 1024-D com BGE-Large ~ 16 min
    dim = compute_sustained_1024d_embeddings(chunks, device)
    
    # 5. Exportação dos Binários
    export_master_artifacts(chunks, dim)''',
    '''    # 4. Vetorização 1024-D com BGE-Large ~ 16 min
    embeddings, dim = compute_sustained_1024d_embeddings(chunks, device)
    
    # 5. Exportação dos Binários
    export_master_artifacts(chunks, embeddings, dim)'''
)

header = f'''# -*- coding: utf-8 -*-
"""
SUPREME MULTI-GAME BOT BRAIN & POLICY NETWORK
==============================================
Treinamento Continuo de Alta Densidade (1 Hora+) em GPU Kaggle (Tesla T4 / P100)
- Dados Reais: Huntera, PokeIdle, Baiak, Poker, AI Go Browser, Agent Go
- SEM TIBIA (totalmente excluido)
- Arquitetura:
  1. DeepBotPolicyNetwork (5 camadas, 1.200 epocas, AdamW + OneCycleLR) ~32 min
  2. Torneio Evolutivo (500k bots x 100 geracoes) ~15 min
  3. Sintese de 200.000 Procedimentos de Combate & Navegacao Real
  4. Vetorizacao 1024-D com BAAI/bge-large-en-v1.5 FP16 ~16 min
  5. Exportacao Binaria de Alta Performance para Go (.bin)
"""

import os
import sys
import time
import math
import struct
import random
import base64
import gzip
import json
import subprocess
from pathlib import Path

try:
    import torch
    import torch.nn as nn
    import torch.optim as optim
    import numpy as np
    from sentence_transformers import SentenceTransformer
except ImportError:
    print("[1/6] Instalando PyTorch, NumPy e sentence-transformers...")
    subprocess.check_call([sys.executable, "-m", "pip", "install", "-q", "torch", "numpy", "sentence-transformers"])
    import torch
    import torch.nn as nn
    import torch.optim as optim
    import numpy as np
    from sentence_transformers import SentenceTransformer

# 2. Dados Reais Extraidos dos Bots (Huntera, PokeIdle, Baiak, Poker, Browser, Agent Go)
_B64_DATA = "{compressed_b64}"
REAL_GAME_DATA = json.loads(gzip.decompress(base64.b64decode(_B64_DATA)).decode("utf-8"))
print(f"Carregados com sucesso {{len(REAL_GAME_DATA)}} nos mestres de dados reais dos bots!")

'''

full_script = header + rest_of_code

with open("kaggle/build_bot_brain.py", "w", encoding="utf-8") as f:
    f.write(full_script)

print(f"Generated build_bot_brain.py successfully ({len(full_script)} bytes)")
