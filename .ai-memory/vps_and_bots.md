# Ambiente VPS & Inventário Completo de Bots

#vps #bots #oracle-cloud #inventory

- **Servidor**: Oracle Cloud VPS (129.153.157.17), CPU AMD EPYC (Zen 1), 951MB RAM física total.
- **Vigilância**: Watchdog nativo rodando a cada 45s para reiniciar processos caídos e manter consumo sob controle.
- **Memória Disponível**: ~558MB livres/disponíveis (OpenCode e processos zumbis eliminados).

---

## 🤖 Inventário dos 15 Bots Ativos em Produção

| Porta | Bot | Caminho na VPS | Consumo RAM Médio |
|---|---|---|---|
| **3000** | **Huntera Bot** | `/home/ubuntu/huntera-go/huntera-bot` | ~11.0 MB |
| **3001** | **Tibidle Bot** | `/home/ubuntu/tibidle-bot/tibidle-bot` | ~10.1 MB |
| **3002** | **Sigma Bot** | `/home/ubuntu/sigma-bot/bot-sigma-linux` | ~12.2 MB |
| **3003** | **Leveling Bot** | `/home/ubuntu/leveling-bot/bot-leveling` | ~11.5 MB |
| **3005** | **PokeLegacy Bot** | `/home/ubuntu/pokelegacybot/pokelegacy-linux` | ~11.3 MB |
| **3006** | **PokeIdle Bot** | `/home/ubuntu/pokeidle-bot/bot-poke` | ~6.3 MB |
| **3007** | **PokeIdleWorld Bot** | `/home/ubuntu/pokeidleworld-bot/bot-poke` | ~10.8 MB |
| **3009** | **Kick Miner** | `/home/ubuntu/kick-miner/kick-miner-linux` | ~10.5 MB |
| **3010** | **Twitch Miner** | `/home/ubuntu/twitch-miner/twitch-miner-linux` | ~6.5 MB |
| **3011** | **LoL Idle Bot** | `/home/ubuntu/lolidle-bot/bot-lolidle-linux` | ~4.1 MB |
| **3012** | **Lux Idle Bot** | `/home/ubuntu/luxidle-bot/luxidle-linux` | ~10.6 MB |
| **3013** | **FarmBot Ricochet** | `/home/ubuntu/farmbot/farmbot-linux` | ~8.7 MB |
| **3014** | **PokeTrue Bot** | `/home/ubuntu/poketrue-bot/poketrue-bot` | ~12.5 MB |
| **3015** | **PokePixel Bot** | `/home/ubuntu/pokepixel-bot/pokepixel-bot-linux` | ~8.9 MB |
| **8090** | **Baiak Idle Bot** | `/home/ubuntu/baiak-bot-ts/.../baiak-bot-linux` | ~12.8 MB |

---

## ⚡ Servidores & Hubs de Controle
- **Porta 8888**: `floppy-server` (Hub de Controle Web & Telemetria)
- **Porta 8899**: `agent-go-test`
