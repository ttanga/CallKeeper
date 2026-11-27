import unittest
from callkeeper import parse_contacts

class Contacts(unittest.TestCase):
    def test_available(self):
        self.assertEqual(parse_contacts("Contact: 1001/sip:a hash Avail 12.0\nContact: 1002/sip:b hash Unavail nan",["1001","1002","1003"]),{"1001":True,"1002":False,"1003":False})
    def test_empty(self):
        self.assertEqual(parse_contacts("No objects found.",["1001"]),{"1001":False})
    def test_invalid(self):
        with self.assertRaises(RuntimeError):
            parse_contacts("error",["1001"])

if __name__=="__main__":
    unittest.main()
