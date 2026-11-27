import json
import secrets
import os
import logging
import re
import sqlite3
import subprocess
import threading
import time
import urllib.request
from pathlib import Path

ROOT = Path("/apps/callkeeper")
LOG = logging.getLogger("callkeeper")

def cli(command):
    result = subprocess.run(["/usr/sbin/asterisk", "-rx", command],
                            capture_output=True, text=True, timeout=12)
    if result.returncode or "Unable to connect" in result.stdout + result.stderr:
        raise RuntimeError("Asterisk CLI unavailable")
    return result.stdout

def parse_contacts(output, ids):
    if "Contact:" not in output and not re.search(r"Objects found:\s*0|No objects found", output):
        raise RuntimeError("Unexpected contact output")
    status = dict.fromkeys(ids, False)
    for line in output.splitlines():
        fields = line.split()
        if len(fields) >= 5 and fields[0] == "Contact:":
            key = fields[1].split("/", 1)[0]
            if key in status and fields[3] == "Avail":
                status[key] = True
    return status

class App:
    def __init__(self, config, db):
        self.c = config
        self.phones = dict(config["phones"])
        self.targets = [k for k,v in self.phones.items() if v["target"]]
        if not self.targets or config["default_target"] not in self.targets:
            raise ValueError("Invalid default target")
        if any(not re.fullmatch(r"[0-9]{1,10}", k) for k in self.phones):
            raise ValueError("Invalid SIP ID")
        if not config["user_ids"] or not config["chat_id"] or config["bot_token"] == "REPLACE":
            raise ValueError("Configure Telegram credentials and authorized IDs")
        self.db = db
        self.lock = threading.RLock()
        with self.connect() as con:
            con.executescript("""
            CREATE TABLE IF NOT EXISTS devices (id TEXT PRIMARY KEY, name TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS deletions (message_id INTEGER PRIMARY KEY, due REAL NOT NULL);
            CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT);
            CREATE TABLE IF NOT EXISTS state (id TEXT PRIMARY KEY, down REAL, alerted INTEGER NOT NULL);
            CREATE TABLE IF NOT EXISTS history (id TEXT, started REAL, ended REAL);
            """)
        self.refresh_devices()
        self.pending = False
    def refresh_devices(self):
        with self.connect() as con:
            devices = con.execute("SELECT id,name FROM devices").fetchall()
        self.phones = dict(self.c["phones"])
        self.phones.update({k:{"name":n,"target":True} for k,n in devices})
        self.targets = [k for k,v in self.phones.items() if v["target"]]
    def connect(self):
        return sqlite3.connect(self.db, timeout=10)
    def get(self, key, default):
        with self.connect() as con:
            row = con.execute("SELECT value FROM settings WHERE key=?", (key,)).fetchone()
        return row[0] if row else default
    def put(self, key, value):
        with self.connect() as con:
            con.execute("INSERT OR REPLACE INTO settings VALUES (?,?)", (key,str(value)))
    def api(self, method, **data):
        req = urllib.request.Request("https://api.telegram.org/bot"+self.c["bot_token"]+"/"+method,
            data=json.dumps(data).encode(), headers={"Content-Type":"application/json"})
        try:
            with urllib.request.urlopen(req, timeout=40) as response:
                obj = json.load(response)
            if not obj["ok"]:
                raise RuntimeError("Telegram rejected request")
            return obj["result"]
        except Exception:
            raise RuntimeError("Telegram API request failed") from None
    def send(self, text, keyboard=None):
        args = {"chat_id":self.c["chat_id"],"text":text}
        if keyboard:
            args["reply_markup"]={"inline_keyboard":keyboard}
        return self.api("sendMessage", **args)
    def create_phone(self, name):
        name = name.strip()
        if not 1 <= len(name) <= 60 or any(ord(c)<32 for c in name):
            raise ValueError("Invalid name")
        if not self.c.get("provision_enabled",False):
            self.send("Создание аккаунтов отключено: требуется настройка Asterisk.")
            return
        password = secrets.token_hex(32)
        # Helper accepts JSON on stdin; credentials never appear in argv.
        result = subprocess.run(["/usr/bin/sudo","-n","/usr/local/sbin/callkeeper-provision"],
            input=json.dumps({"password":password}),text=True,capture_output=True,timeout=90)
        if result.returncode or not re.fullmatch(r"\d{4,10}",result.stdout.strip()):
            raise RuntimeError("Provisioning not confirmed")
        key=result.stdout.strip()
        with self.connect() as con:
            con.execute("INSERT INTO devices VALUES (?,?)",(key,name))
        self.refresh_devices()
        message=self.api("sendMessage",chat_id=self.c["chat_id"],protect_content=True,
            text=f"{name} — {key}\nСервер: {self.c['sip_host']}:{self.c['sip_port']}\nTLS / SRTP\nПароль: {password}")
        with self.connect() as con:
            con.execute("INSERT INTO deletions VALUES (?,?)",(message["message_id"],time.time()+self.c.get("password_message_ttl",60)))
        self.menu()
    def cleanup(self):
        while True:
            try:
                with self.connect() as con:
                    rows=con.execute("SELECT message_id FROM deletions WHERE due<=?",(time.time(),)).fetchall()
                for (mid,) in rows:
                    try:
                        self.api("deleteMessage",chat_id=self.c["chat_id"],message_id=mid)
                    except Exception:
                        continue
                    with self.connect() as con:
                        con.execute("DELETE FROM deletions WHERE message_id=?",(mid,))
            except Exception:
                LOG.warning("Password message cleanup failed")
            time.sleep(2)
    def target(self):
        out = cli("database get gateway target")
        match = re.search(r"Value:\s*(\d+)\s*$", out, re.M)
        if match:
            return match.group(1)
        if "not exist" in out.lower():
            return self.c["default_target"]
        raise RuntimeError("Cannot read route")
    def set_target(self, key):
        if key not in self.targets:
            raise ValueError("Target not allowed")
        out = cli("pjsip show endpoint "+key)
        if not re.search(r"Endpoint:\s+"+re.escape(key)+r"(?:\s|/)", out):
            raise RuntimeError("Endpoint not found")
        cli("database put gateway target "+key)
        if self.target() != key:
            raise RuntimeError("Route not confirmed")
    def snapshot(self):
        return parse_contacts(cli("pjsip show contacts"), self.phones)
    def label(self, key):
        return key+" — "+self.phones[key]["name"] if key in self.phones else key
    def enabled(self,key):
        return self.get("notify:"+key,"1") == "1"
    def delay(self):
        return int(self.get("delay",self.c["down_delay"]))
    def observe(self,key,up,now):
        with self.connect() as con:
            row=con.execute("SELECT down,alerted FROM state WHERE id=?",(key,)).fetchone()
            down, alerted = row if row else (None,0)
            if up:
                if down is not None and alerted:
                    self.send("🟢 Восстановлен: "+self.label(key)+"; "+str(int(now-down))+" сек.")
                if down is not None:
                    con.execute("INSERT INTO history VALUES (?,?,?)",(key,down,now))
                down,alerted=None,0
            else:
                if down is None:
                    down=now
                if not alerted and self.enabled(key) and now-down >= self.delay():
                    self.send("🔴 Недоступен: "+self.label(key))
                    alerted=1
            con.execute("INSERT OR REPLACE INTO state VALUES (?,?,?)",(key,down,alerted))
    def monitor(self):
        while True:
            try:
                if self.get("monitor",str(int(self.c["monitor_enabled"])))=="1":
                    try:
                        statuses=self.snapshot()
                    except Exception:
                        self.observe("Asterisk",False,time.time())
                    else:
                        now=time.time()
                        self.observe("Asterisk",True,now)
                        for key,up in statuses.items():
                            self.observe(key,up,now)
            except Exception:
                LOG.warning("Monitoring iteration failed")
            time.sleep(self.c["poll_seconds"])
    def menu(self):
        try:
            target=self.target()
            status=self.snapshot()
            lines=["CallKeeper","🟢 Asterisk","Получатель: "+self.label(target)]
            lines += [("🟢 " if up else "🔴 ")+self.label(k) for k,up in status.items()]
        except Exception:
            target=None
            lines=["CallKeeper","⚠️ Asterisk CLI недоступен или ответ не распознан"]
        buttons=[[{"text":("✓ " if k==target else "")+self.label(k),"callback_data":"target:"+k}] for k in self.targets]
        buttons += [[{"text":"Добавить телефон","callback_data":"add"}], [{"text":"Тестовый звонок","callback_data":"test_confirm"},{"text":"Обновить","callback_data":"menu"}],
                    [{"text":"Уведомления","callback_data":"settings"},{"text":"История","callback_data":"history"}]]
        self.send("\n".join(lines),buttons)
    def settings(self):
        keys=["Asterisk"]+list(self.phones)
        buttons=[[{"text":("🔔 " if self.enabled(k) else "🔕 ")+self.label(k),"callback_data":"notify:"+k}] for k in keys]
        buttons += [[{"text":str(n)+" сек.","callback_data":"delay:"+str(n)} for n in (30,60,120,300)],
                    [{"text":"Мониторинг вкл.","callback_data":"monitor:1"},{"text":"Мониторинг выкл.","callback_data":"monitor:0"}],
                    [{"text":"Назад","callback_data":"menu"}]]
        self.send("Задержка: "+str(self.delay())+" сек.\nМониторинг: "+self.get("monitor",str(int(self.c["monitor_enabled"]))),buttons)
    def authorized(self, user, chat):
        return user in self.c["user_ids"] and chat == self.c["chat_id"]
    def handle(self, update):
        cb=update.get("callback_query")
        if cb:
            if cb.get("message",{}).get("chat",{}).get("type") != "private" or not self.authorized(cb.get("from",{}).get("id"),cb.get("message",{}).get("chat",{}).get("id")):
                self.api("answerCallbackQuery",callback_query_id=cb["id"],text="Нет доступа")
                return
            self.api("answerCallbackQuery",callback_query_id=cb["id"])
            action=cb.get("data","")
            if action=="add":
                self.pending=True
                self.send("Введите название нового телефона или /cancel")
            elif action.startswith("target:"):
                self.set_target(action.split(":",1)[1])
                self.menu()
            elif action=="menu":
                self.menu()
            elif action=="settings":
                self.settings()
            elif action=="test_confirm":
                self.send("Позвонить на текущий маршрут?",[[{"text":"Подтвердить","callback_data":"test"},{"text":"Отмена","callback_data":"menu"}]])
            elif action=="test":
                if self.target() not in self.targets:
                    raise RuntimeError("Invalid target")
                if not re.search(r"2000\s*=>|['\"]2000['\"]",cli("dialplan show 2000@from-gateway")):
                    raise RuntimeError("Route 2000 not found")
                cli("channel originate Local/2000@from-gateway application Echo")
                self.send("Команда тестового звонка отправлена. Проверь телефон.")
            elif action.startswith("notify:"):
                key=action.split(":",1)[1]
                if key in self.phones or key=="Asterisk":
                    self.put("notify:"+key,int(not self.enabled(key)))
                    self.settings()
            elif action in ("delay:30","delay:60","delay:120","delay:300"):
                self.put("delay",action.split(":")[1])
                self.settings()
            elif action in ("monitor:0","monitor:1"):
                self.put("monitor",action[-1])
                self.settings()
            elif action=="history":
                with self.connect() as con:
                    rows=con.execute("SELECT id,started,ended FROM history ORDER BY ended DESC LIMIT 15").fetchall()
                self.send("\n".join(self.label(k)+" | "+time.strftime("%Y-%m-%d %H:%M",time.localtime(s))+" | "+str(int(e-s))+" сек." for k,s,e in rows) or "История пуста")
        else:
            msg=update.get("message",{})
            if msg.get("chat",{}).get("type") != "private" or not self.authorized(msg.get("from",{}).get("id"),msg.get("chat",{}).get("id")):
                return
            value=msg.get("text","")
            if value in ("/start","/menu","/status","/cancel"):
                self.pending=False
                self.menu()
            elif self.pending and value and not value.startswith("/"):
                self.pending=False
                self.create_phone(value)
    def run(self):
        threading.Thread(target=self.monitor,daemon=True).start()
        threading.Thread(target=self.cleanup,daemon=True).start()
        offset=int(self.get("offset","0"))
        while True:
            try:
                updates=self.api("getUpdates",offset=offset,timeout=25,allowed_updates=["message","callback_query"])
                for update in updates:
                    offset=update["update_id"]+1
                    self.put("offset",offset)
                    try:
                        self.handle(update)
                    except Exception:
                        LOG.warning("Command failed")
                        self.send("⚠️ Команда не выполнена или результат не подтверждён.")
            except Exception:
                LOG.warning("Polling failed; retrying")
                time.sleep(5)

if __name__=="__main__":
    logging.basicConfig(level=logging.INFO)
    config=json.loads((ROOT/"config.json").read_text())
    App(config,ROOT/"state"/"callkeeper.sqlite").run()
