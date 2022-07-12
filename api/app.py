from flask import Flask
import ety
import json

app = Flask(__name__)

@app.route("/api/ety")
def hello_world():
    inputWords = 'this is a test sentence to try out the system'
    outputWords = []

    for word in inputWords.split():
        et = ety.origins(word)
        etm = None

        if len(et) > 0:
            etm = et[0].language.__dict__

        outputWords.append({
            'word': word,
            'et': etm
        })

    return json.dumps(outputWords)
    # words = ety.origins('potato')
    # if len(words) > 0:
    #     return json.dumps(words[0].language.__dict__)
    # else:
    #     return 'not found'