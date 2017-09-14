import base64
import json
import os
import six
import sys
import traceback

# we define our own StringIO object because StringIO.StringIO isn't
# available in Python 3 but io.StringIO in Python 2 raises an exception
# if a non-unicode string is passed to the write method, so it can't be
# used with traceback.print_exc
class StringIO(object):
	def __init__(self):
		self.s = ''

	def write(self, s):
		self.s += s

	def getvalue(self):
		return self.s

vars={}
while True:
	line=sys.stdin.readline()
	if line == "":
		break
	result={}
	try:
		vars["result"] = None
		six.exec_(base64.b64decode(line), globals(), vars)
		result["result"] = vars["result"]
	except:
		f = StringIO()
		traceback.print_exc(file=f)
		result["exception"] = f.getvalue()
	out = json.dumps(result)
	send = base64.b64encode(out.encode('utf-8')).decode('utf-8') + '\n'
	sys.stdout.write(send)
	sys.stdout.flush()

